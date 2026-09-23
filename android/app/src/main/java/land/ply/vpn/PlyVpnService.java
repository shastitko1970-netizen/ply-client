package land.ply.vpn;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.content.Intent;
import android.net.VpnService;
import android.os.Build;
import android.os.ParcelFileDescriptor;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.util.ArrayList;

public class PlyVpnService extends VpnService {
    private ParcelFileDescriptor tun;
    private Process xray;

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        if (intent != null && "land.ply.vpn.STOP".equals(intent.getAction())) {
            stopSelf();
            return START_NOT_STICKY;
        }
        startForeground(1, note());
        String url = intent != null ? intent.getStringExtra(MainActivity.EXTRA_URL) : "";
        boolean split = intent == null || intent.getBooleanExtra(MainActivity.EXTRA_SPLIT, true);
        int mtu = intent != null ? intent.getIntExtra(MainActivity.EXTRA_MTU, 1400) : 1400;
        if (mtu != 1280 && mtu != 1400 && mtu != 1500) mtu = 1400;
        try {
            stopTunnel();
            File dir = new File(getFilesDir(), "xray");
            dir.mkdirs();
            File bin = copyBin(dir, "xray");
            copyAsset("geoip.dat", new File(dir, "geoip.dat"));
            copyAsset("geosite.dat", new File(dir, "geosite.dat"));
            Builder b = new Builder();
            b.setSession("Ply");
            b.setMtu(mtu);
            b.addAddress("198.18.0.1", 16);
            b.addDnsServer("1.1.1.1");
            b.addRoute("0.0.0.0", 1);
            b.addRoute("128.0.0.0", 1);
            try { b.addDisallowedApplication(getPackageName()); } catch (Exception ignored) {}
            tun = b.establish();
            if (tun == null) {
                stopSelf();
                return START_NOT_STICKY;
            }
            File cfg = new File(dir, "config.json");
            File plycfg = copyBin(dir, "plycfg");
            ProcessBuilder gen = new ProcessBuilder(
                plycfg.getAbsolutePath(),
                "-url", url == null ? "" : url,
                "-split=" + (split ? "true" : "false"),
                "-fd", String.valueOf(tun.getFd()),
                "-mtu", String.valueOf(mtu),
                "-out", cfg.getAbsolutePath()
            );
            gen.directory(dir);
            Process g = gen.start();
            if (g.waitFor() != 0) {
                writeFallback(cfg, tun.getFd());
            }
            ArrayList<String> cmd = new ArrayList<String>();
            cmd.add(bin.getAbsolutePath());
            cmd.add("run");
            cmd.add("-c");
            cmd.add(cfg.getAbsolutePath());
            ProcessBuilder pb = new ProcessBuilder(cmd);
            pb.directory(dir);
            pb.environment().put("XRAY_TUN_FD", String.valueOf(tun.getFd()));
            pb.redirectErrorStream(true);
            xray = pb.start();
        } catch (Exception e) {
            stopSelf();
        }
        return START_STICKY;
    }

    @Override
    public void onDestroy() {
        stopTunnel();
        super.onDestroy();
    }

    @Override
    public void onRevoke() {
        stopTunnel();
        super.onRevoke();
    }

    private void stopTunnel() {
        if (xray != null) {
            xray.destroy();
            xray = null;
        }
        if (tun != null) {
            try { tun.close(); } catch (Exception ignored) {}
            tun = null;
        }
    }

    private File copyBin(File dir, String name) throws Exception {
        File dest = new File(dir, name);
        if (!dest.exists() || dest.length() < 100) {
            copyAsset(name, dest);
        }
        dest.setExecutable(true);
        return dest;
    }

    private void copyAsset(String name, File dest) throws Exception {
        InputStream in = getAssets().open(name);
        FileOutputStream out = new FileOutputStream(dest);
        byte[] buf = new byte[8192];
        int n;
        while ((n = in.read(buf)) > 0) out.write(buf, 0, n);
        out.close();
        in.close();
    }

    private void writeFallback(File cfg, int fd) throws Exception {
        String body = "{\n" +
            "  \"log\": {\"loglevel\": \"warning\"},\n" +
            "  \"inbounds\": [{\n" +
            "    \"tag\": \"tun\", \"protocol\": \"tun\",\n" +
            "    \"settings\": {\"mtu\": 1400, \"fd\": " + fd + "},\n" +
            "    \"sniffing\": {\"enabled\": true, \"destOverride\": [\"http\", \"tls\", \"quic\"], \"routeOnly\": true}\n" +
            "  }],\n" +
            "  \"outbounds\": [{\"tag\": \"direct\", \"protocol\": \"freedom\"}]\n" +
            "}\n";
        FileOutputStream out = new FileOutputStream(cfg);
        out.write(body.getBytes("UTF-8"));
        out.close();
    }

    private Notification note() {
        String ch = "ply";
        if (Build.VERSION.SDK_INT >= 26) {
            NotificationChannel c = new NotificationChannel(ch, "Ply", NotificationManager.IMPORTANCE_LOW);
            NotificationManager nm = getSystemService(NotificationManager.class);
            if (nm != null) nm.createNotificationChannel(c);
        }
        Notification.Builder b;
        if (Build.VERSION.SDK_INT >= 26) b = new Notification.Builder(this, ch);
        else b = new Notification.Builder(this);
        return b.setContentTitle("Ply")
            .setContentText("VPN включён")
            .setSmallIcon(android.R.drawable.ic_lock_lock)
            .setOngoing(true)
            .build();
    }
}
