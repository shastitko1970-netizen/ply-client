package land.ply.vpn;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.content.Intent;
import android.net.VpnService;
import android.os.Build;
import android.os.ParcelFileDescriptor;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.util.ArrayList;

public class PlyVpnService extends VpnService {
    private ParcelFileDescriptor tun;
    private int tunFd = -1;
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
        if (mtu != 1200 && mtu != 1280 && mtu != 1400 && mtu != 1500) mtu = 1400;
        String rawUrl = url == null ? "" : url;
        if ((rawUrl.contains("hy2://") || rawUrl.contains("hysteria2://")) && mtu > 1200) mtu = 1200;
        String bypass = intent != null ? intent.getStringExtra(MainActivity.EXTRA_BYPASS) : "";
        if (bypass == null) bypass = "";
        try {
            stopTunnel();
            File dir = new File(getFilesDir(), "xray");
            dir.mkdirs();
            freshBins(dir);
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
            if (Build.VERSION.SDK_INT >= 29) b.setMetered(false);
            try { b.addDisallowedApplication(getPackageName()); } catch (Exception ignored) {}
            String[] apps = bypass.split("\n");
            for (int i = 0; i < apps.length; i++) {
                String pkg = apps[i].trim();
                if (pkg.length() == 0 || pkg.equals(getPackageName())) continue;
                try { b.addDisallowedApplication(pkg); } catch (Exception ignored) {}
            }
            tun = b.establish();
            if (tun == null) {
                stopSelf();
                return START_NOT_STICKY;
            }
            tunFd = tun.detachFd();
            String node = intent != null ? intent.getStringExtra(MainActivity.EXTRA_NODE) : "";
            if (node == null) node = "";
            File cfg = new File(dir, "config.json");
            File plycfg = copyBin(dir, "plycfg");
            ProcessBuilder gen = new ProcessBuilder(
                plycfg.getAbsolutePath(),
                "-url", rawUrl,
                "-split=" + (split ? "true" : "false"),
                "-fd", String.valueOf(tunFd),
                "-mtu", String.valueOf(mtu),
                "-node", node,
                "-bypass", bypass,
                "-out", cfg.getAbsolutePath()
            );
            gen.directory(dir);
            gen.redirectErrorStream(true);
            Process g = gen.start();
            String genOut = readAll(g.getInputStream());
            if (g.waitFor() != 0) {
                notifyFail(genOut);
                stopTunnel();
                stopSelf();
                return START_NOT_STICKY;
            }
            ArrayList<String> cmd = new ArrayList<String>();
            cmd.add(bin.getAbsolutePath());
            cmd.add("run");
            cmd.add("-c");
            cmd.add(cfg.getAbsolutePath());
            ProcessBuilder pb = new ProcessBuilder(cmd);
            pb.directory(dir);
            pb.environment().put("XRAY_TUN_FD", String.valueOf(tunFd));
            pb.redirectErrorStream(true);
            xray = pb.start();
        } catch (Exception e) {
            notifyFail(e.getMessage());
            stopTunnel();
            stopSelf();
            return START_NOT_STICKY;
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
        if (tunFd >= 0) {
            try { ParcelFileDescriptor.adoptFd(tunFd).close(); } catch (Exception ignored) {}
            tunFd = -1;
        }
    }

    private void freshBins(File dir) throws Exception {
        File stamp = new File(dir, "stamp");
        String have = "";
        if (stamp.exists()) {
            java.io.FileInputStream in = new java.io.FileInputStream(stamp);
            byte[] buf = new byte[32];
            int n = in.read(buf);
            in.close();
            if (n > 0) have = new String(buf, 0, n, "UTF-8").trim();
        }
        if (!"2.1.4".equals(have)) {
            copyAsset("xray", new File(dir, "xray"));
            copyAsset("plycfg", new File(dir, "plycfg"));
            copyAsset("geoip.dat", new File(dir, "geoip.dat"));
            copyAsset("geosite.dat", new File(dir, "geosite.dat"));
            FileOutputStream o = new FileOutputStream(stamp);
            o.write("2.1.4".getBytes("UTF-8"));
            o.close();
        } else {
            copyAsset("plycfg", new File(dir, "plycfg"));
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

    private void notifyFail(String msg) {
        if (msg == null || msg.trim().length() == 0) msg = "ключ не разобрался";
        msg = msg.trim().replace('\n', ' ');
        if (msg.length() > 140) msg = msg.substring(0, 140);
        String ch = "ply";
        if (Build.VERSION.SDK_INT >= 26) {
            NotificationChannel c = new NotificationChannel(ch, "Ply", NotificationManager.IMPORTANCE_LOW);
            NotificationManager nm = getSystemService(NotificationManager.class);
            if (nm != null) nm.createNotificationChannel(c);
        }
        Notification.Builder b;
        if (Build.VERSION.SDK_INT >= 26) b = new Notification.Builder(this, ch);
        else b = new Notification.Builder(this);
        Notification n = b.setContentTitle("Ply")
            .setContentText(msg)
            .setSmallIcon(android.R.drawable.ic_lock_lock)
            .build();
        NotificationManager nm = (NotificationManager) getSystemService(NOTIFICATION_SERVICE);
        if (nm != null) nm.notify(2, n);
    }

    private static String readAll(InputStream in) throws Exception {
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        byte[] buf = new byte[4096];
        int n;
        while ((n = in.read(buf)) > 0) out.write(buf, 0, n);
        in.close();
        return out.toString("UTF-8");
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
