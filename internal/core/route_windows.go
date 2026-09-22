//go:build windows

package core

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	iphlpapi      = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetBestIf = iphlpapi.NewProc("GetBestInterface")
	routeMu       sync.Mutex
)

type ifInfo struct {
	Index uint32
	Name  string
	Desc  string
}

func plyAdapter() *ifInfo {
	var size uint32 = 15000
	for i := 0; i < 3; i++ {
		buf := make([]byte, size)
		aa := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, windows.GAA_FLAG_INCLUDE_PREFIX, 0, aa, &size)
		if err == windows.ERROR_BUFFER_OVERFLOW {
			continue
		}
		if err != nil {
			return nil
		}
		for ; aa != nil; aa = aa.Next {
			desc := windows.UTF16PtrToString(aa.Description)
			name := windows.UTF16PtrToString(aa.FriendlyName)
			blob := strings.ToLower(desc + " " + name)
			if strings.Contains(blob, "ply") && aa.OperStatus == windows.IfOperStatusUp {
				return &ifInfo{Index: aa.IfIndex, Name: name, Desc: desc}
			}
		}
		return nil
	}
	return nil
}

func bestIfIndex(ip net.IP) uint32 {
	v4 := ip.To4()
	if v4 == nil {
		return 0
	}
	dest := binary.BigEndian.Uint32(v4)
	var idx uint32
	r, _, _ := procGetBestIf.Call(uintptr(dest), uintptr(unsafe.Pointer(&idx)))
	if r != 0 {
		return 0
	}
	return idx
}

func DefaultViaPly() bool {
	a := plyAdapter()
	if a == nil {
		return false
	}
	for _, s := range []string{"1.1.1.1", "8.8.8.8", "9.9.9.9"} {
		idx := bestIfIndex(net.ParseIP(s))
		if idx == 0 {
			continue
		}
		return idx == a.Index
	}
	return false
}

func routeStatePath() string {
	dir, err := DataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "route-state.json")
}

func runPS(script string, env []string) (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "net.ps1")
	body := append([]byte{0xEF, 0xBB, 0xBF}, []byte(script)...)
	if err := os.WriteFile(path, body, 0644); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", path)
	tuneCmd(cmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

const applyTunPS = `
$ErrorActionPreference = 'SilentlyContinue'
$stateFile = $env:PLY_STATE
$serverIP = $env:PLY_SERVER
if (-not $stateFile) { Write-Output 'NO_STATE'; exit 2 }

$ply = Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and ($_.InterfaceDescription -match 'Ply' -or $_.Name -match 'Ply') } | Select-Object -First 1
if (-not $ply) { Write-Output 'NO_PLY'; exit 3 }
$idx = [int]$ply.ifIndex

if (-not (Test-Path -LiteralPath $stateFile)) {
  $defs = @(Get-NetRoute -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue | Where-Object { $_.InterfaceIndex -ne $idx } | ForEach-Object {
    @{ IfIndex = $_.InterfaceIndex; NextHop = $_.NextHop; Metric = $_.RouteMetric }
  })
  $metrics = @(Get-NetIPInterface | Where-Object { $_.ConnectionState -eq 'Connected' } | ForEach-Object {
    @{ IfIndex = $_.InterfaceIndex; Family = [int]$_.AddressFamily; Metric = $_.InterfaceMetric }
  })
  $dns = @()
  foreach ($d in $defs) {
    $cfg = Get-DnsClientServerAddress -InterfaceIndex $d.IfIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue
    $dns += @{ IfIndex = $d.IfIndex; Servers = @($cfg.ServerAddresses) }
  }
  $obj = @{ defaults = $defs; metrics = $metrics; dns = $dns; serverIP = $serverIP; plyIf = $idx }
  ($obj | ConvertTo-Json -Depth 6) | Set-Content -LiteralPath $stateFile -Encoding UTF8
}

try { Rename-NetAdapter -Name $ply.Name -NewName 'Ply VPN' -ErrorAction SilentlyContinue } catch {}

Set-NetIPInterface -InterfaceIndex $idx -InterfaceMetric 1 -AddressFamily IPv4 -ErrorAction SilentlyContinue
Set-NetIPInterface -InterfaceIndex $idx -Forwarding Enabled -AddressFamily IPv4 -ErrorAction SilentlyContinue
Set-DnsClientServerAddress -InterfaceIndex $idx -ServerAddresses @('1.1.1.1','8.8.8.8') -ErrorAction SilentlyContinue

Get-NetIPInterface | Where-Object { $_.InterfaceIndex -ne $idx -and $_.ConnectionState -eq 'Connected' } | ForEach-Object {
  Set-NetIPInterface -InterfaceIndex $_.InterfaceIndex -AddressFamily $_.AddressFamily -InterfaceMetric 5000 -ErrorAction SilentlyContinue
}

$state = Get-Content -LiteralPath $stateFile -Raw | ConvertFrom-Json
$primary = $null
if ($state.defaults) {
  $primary = @($state.defaults) | Select-Object -First 1
}
if ($serverIP -and $primary -and $primary.NextHop) {
  route delete $serverIP mask 255.255.255.255 | Out-Null
  if ($primary.NextHop -ne '0.0.0.0') {
    route add $serverIP mask 255.255.255.255 $primary.NextHop metric 1 if ([int]$primary.IfIndex) | Out-Null
  }
}

route delete 0.0.0.0 mask 128.0.0.0 | Out-Null
route delete 128.0.0.0 mask 128.0.0.0 | Out-Null
route add 0.0.0.0 mask 128.0.0.0 198.18.0.1 metric 1 if $idx | Out-Null
route add 128.0.0.0 mask 128.0.0.0 198.18.0.1 metric 1 if $idx | Out-Null

if ($state.dns) {
  foreach ($d in @($state.dns)) {
    Set-DnsClientServerAddress -InterfaceIndex ([int]$d.IfIndex) -ServerAddresses @('1.1.1.1','8.8.8.8') -ErrorAction SilentlyContinue
  }
}

ipconfig /flushdns | Out-Null
Write-Output 'OK'
`

const restoreTunPS = `
$ErrorActionPreference = 'SilentlyContinue'
$stateFile = $env:PLY_STATE
if (-not $stateFile -or -not (Test-Path -LiteralPath $stateFile)) { Write-Output 'NONE'; exit 0 }
$state = Get-Content -LiteralPath $stateFile -Raw | ConvertFrom-Json

route delete 0.0.0.0 mask 128.0.0.0 | Out-Null
route delete 128.0.0.0 mask 128.0.0.0 | Out-Null
if ($state.serverIP) {
  route delete $state.serverIP mask 255.255.255.255 | Out-Null
}

if ($state.metrics) {
  foreach ($m in @($state.metrics)) {
    Set-NetIPInterface -InterfaceIndex ([int]$m.IfIndex) -AddressFamily ([int]$m.Family) -InterfaceMetric ([int]$m.Metric) -ErrorAction SilentlyContinue
  }
}
if ($state.dns) {
  foreach ($d in @($state.dns)) {
    $servers = @($d.Servers)
    if ($servers.Count -eq 0) {
      Set-DnsClientServerAddress -InterfaceIndex ([int]$d.IfIndex) -ResetServerAddresses -ErrorAction SilentlyContinue
    } else {
      Set-DnsClientServerAddress -InterfaceIndex ([int]$d.IfIndex) -ServerAddresses $servers -ErrorAction SilentlyContinue
    }
  }
}
Remove-Item -LiteralPath $stateFile -Force -ErrorAction SilentlyContinue
ipconfig /flushdns | Out-Null
Write-Output 'OK'
`

func ApplyTunRoutes(serverIP string) error {
	routeMu.Lock()
	defer routeMu.Unlock()
	st := routeStatePath()
	if st == "" {
		return fmt.Errorf("нет папки данных")
	}
	out, err := runPS(applyTunPS, []string{
		"PLY_STATE=" + st,
		"PLY_SERVER=" + strings.TrimSpace(serverIP),
	})
	if err != nil {
		return fmt.Errorf("маршруты: %s (%v)", out, err)
	}
	if strings.Contains(out, "NO_PLY") {
		return fmt.Errorf("адаптер Ply не найден")
	}
	if !strings.Contains(out, "OK") {
		return fmt.Errorf("маршруты: %s", out)
	}
	return nil
}

func RestoreTunRoutes() {
	routeMu.Lock()
	defer routeMu.Unlock()
	st := routeStatePath()
	if st == "" {
		return
	}
	_, _ = runPS(restoreTunPS, []string{"PLY_STATE=" + st})
}

func RegisterVpnProfile(serverIP string) {
	if strings.TrimSpace(serverIP) == "" {
		serverIP = "198.18.0.1"
	}
	script := `
$ErrorActionPreference = 'SilentlyContinue'
$server = $env:PLY_SERVER
$existing = Get-VpnConnection -Name 'Ply' -AllUserConnection -ErrorAction SilentlyContinue
if (-not $existing) { $existing = Get-VpnConnection -Name 'Ply' -ErrorAction SilentlyContinue }
if ($existing) {
  Set-VpnConnection -Name 'Ply' -ServerAddress $server -SplitTunneling $true -ErrorAction SilentlyContinue
  Set-VpnConnection -Name 'Ply' -ServerAddress $server -SplitTunneling $true -AllUserConnection -ErrorAction SilentlyContinue
} else {
  Add-VpnConnection -Name 'Ply' -ServerAddress $server -TunnelType Automatic -EncryptionLevel Optional -AuthenticationMethod PAP -SplitTunneling $true -AllUserConnection -Force -RememberCredential:$false -ErrorAction SilentlyContinue | Out-Null
  if (-not (Get-VpnConnection -Name 'Ply' -AllUserConnection -ErrorAction SilentlyContinue) -and -not (Get-VpnConnection -Name 'Ply' -ErrorAction SilentlyContinue)) {
    Add-VpnConnection -Name 'Ply' -ServerAddress $server -TunnelType Automatic -SplitTunneling $true -Force -ErrorAction SilentlyContinue | Out-Null
  }
}
Write-Output 'OK'
`
	_, _ = runPS(script, []string{"PLY_SERVER=" + serverIP})
}

func RemoveVpnProfile() {
	script := `
$ErrorActionPreference = 'SilentlyContinue'
Remove-VpnConnection -Name 'Ply' -Force -AllUserConnection -ErrorAction SilentlyContinue
Remove-VpnConnection -Name 'Ply' -Force -ErrorAction SilentlyContinue
Write-Output 'OK'
`
	_, _ = runPS(script, nil)
}

type routeStateLite struct {
	ServerIP string `json:"serverIP"`
}

func SavedServerIP() string {
	st := routeStatePath()
	b, err := os.ReadFile(st)
	if err != nil {
		return ""
	}
	var s routeStateLite
	if json.Unmarshal(b, &s) != nil {
		return ""
	}
	return s.ServerIP
}
