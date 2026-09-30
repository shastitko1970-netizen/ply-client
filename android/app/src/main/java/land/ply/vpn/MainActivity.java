package land.ply.vpn;

import android.app.Activity;
import android.content.Intent;
import android.content.pm.ApplicationInfo;
import android.content.pm.PackageManager;
import android.content.pm.ResolveInfo;
import android.net.VpnService;
import android.os.Build;
import android.os.Bundle;
import android.webkit.JavascriptInterface;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.List;
import org.json.JSONArray;
import org.json.JSONObject;

public class MainActivity extends Activity {
    public static final String EXTRA_URL = "land.ply.vpn.URL";
    public static final String EXTRA_SPLIT = "land.ply.vpn.SPLIT";
    public static final String EXTRA_MTU = "land.ply.vpn.MTU";
    public static final String EXTRA_NODE = "land.ply.vpn.NODE";
    public static final String EXTRA_BYPASS = "land.ply.vpn.BYPASS";

    private String pendingUrl = "";
    private boolean pendingSplit = true;
    private int pendingMtu = 1400;
    private String pendingNode = "";
    private String pendingBypass = "";
    private String appsJson;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        WebView w = new WebView(this);
        w.setBackgroundColor(0xFF0A0A0B);
        WebSettings s = w.getSettings();
        s.setJavaScriptEnabled(true);
        s.setDomStorageEnabled(true);
        s.setAllowFileAccess(true);
        w.setWebViewClient(new WebViewClient());
        w.addJavascriptInterface(new Bridge(), "PlyNative");
        w.loadUrl("file:///android_asset/index.html");
        setContentView(w);
    }

    class Bridge {
        @JavascriptInterface
        public void connect(final String url, final boolean split) {
            connect(url, split, 1400);
        }

        @JavascriptInterface
        public void connect(final String url, final boolean split, final int mtu) {
            connect(url, split, mtu, "");
        }

        @JavascriptInterface
        public void connect(final String url, final boolean split, final int mtu, final String node) {
            connect(url, split, mtu, node, "");
        }

        @JavascriptInterface
        public void connect(final String url, final boolean split, final int mtu, final String node, final String bypass) {
            runOnUiThread(new Runnable() {
                @Override public void run() {
                    pendingUrl = url == null ? "" : url;
                    pendingSplit = split;
                    pendingMtu = mtu == 1200 || mtu == 1280 || mtu == 1500 ? mtu : 1400;
                    pendingNode = node == null ? "" : node;
                    pendingBypass = bypass == null ? "" : bypass;
                    Intent prep = VpnService.prepare(MainActivity.this);
                    if (prep != null) {
                        startActivityForResult(prep, 77);
                    } else {
                        startVpn();
                    }
                }
            });
        }

        @JavascriptInterface
        public String listApps() {
            if (appsJson != null) return appsJson;
            appsJson = loadApps();
            return appsJson;
        }

        @JavascriptInterface
        public String nodes(String url) {
            try {
                File bin = ensureBin("plycfg");
                ProcessBuilder pb = new ProcessBuilder(
                    bin.getAbsolutePath(),
                    "-list",
                    "-url",
                    url == null ? "" : url
                );
                pb.directory(bin.getParentFile());
                pb.redirectErrorStream(false);
                Process p = pb.start();
                String err = readAll(p.getErrorStream());
                String out = readAll(p.getInputStream());
                int code = p.waitFor();
                if (code != 0) {
                    return "{\"error\":" + jsonQuote(err.length() == 0 ? "ключ не разобрался" : err) + "}";
                }
                return out;
            } catch (Exception e) {
                return "{\"error\":" + jsonQuote(String.valueOf(e.getMessage())) + "}";
            }
        }

        @JavascriptInterface
        public String refresh(String url) {
            try {
                File bin = ensureBin("plycfg");
                ProcessBuilder pb = new ProcessBuilder(
                    bin.getAbsolutePath(),
                    "-pull",
                    "-url",
                    url == null ? "" : url
                );
                pb.directory(bin.getParentFile());
                pb.redirectErrorStream(false);
                Process p = pb.start();
                String err = readAll(p.getErrorStream());
                String out = readAll(p.getInputStream());
                int code = p.waitFor();
                if (code != 0) {
                    return "{\"error\":" + jsonQuote(err.length() == 0 ? "подписка не обновилась" : err) + "}";
                }
                return out;
            } catch (Exception e) {
                return "{\"error\":" + jsonQuote(String.valueOf(e.getMessage())) + "}";
            }
        }

        @JavascriptInterface
        public void disconnect() {
            runOnUiThread(new Runnable() {
                @Override public void run() {
                    Intent i = new Intent(MainActivity.this, PlyVpnService.class);
                    i.setAction("land.ply.vpn.STOP");
                    startService(i);
                }
            });
        }
    }

    private void startVpn() {
        Intent run = new Intent(this, PlyVpnService.class);
        run.putExtra(EXTRA_URL, pendingUrl);
        run.putExtra(EXTRA_SPLIT, pendingSplit);
        run.putExtra(EXTRA_MTU, pendingMtu);
        run.putExtra(EXTRA_NODE, pendingNode == null ? "" : pendingNode);
        run.putExtra(EXTRA_BYPASS, pendingBypass == null ? "" : pendingBypass);
        if (Build.VERSION.SDK_INT >= 26) startForegroundService(run);
        else startService(run);
    }

    private String loadApps() {
        try {
            PackageManager pm = getPackageManager();
            Intent main = new Intent(Intent.ACTION_MAIN);
            main.addCategory(Intent.CATEGORY_LAUNCHER);
            List<ResolveInfo> infos = pm.queryIntentActivities(main, 0);
            ArrayList<JSONObject> rows = new ArrayList<JSONObject>();
            ArrayList<String> seen = new ArrayList<String>();
            String self = getPackageName();
            for (int i = 0; i < infos.size(); i++) {
                ApplicationInfo ai = infos.get(i).activityInfo.applicationInfo;
                if (ai == null || self.equals(ai.packageName)) continue;
                if (seen.contains(ai.packageName)) continue;
                seen.add(ai.packageName);
                CharSequence label = ai.loadLabel(pm);
                JSONObject o = new JSONObject();
                o.put("pkg", ai.packageName);
                o.put("label", label == null ? ai.packageName : label.toString());
                rows.add(o);
            }
            Collections.sort(rows, new Comparator<JSONObject>() {
                @Override public int compare(JSONObject a, JSONObject b) {
                    return a.optString("label").compareToIgnoreCase(b.optString("label"));
                }
            });
            JSONArray arr = new JSONArray();
            for (int i = 0; i < rows.size(); i++) arr.put(rows.get(i));
            return arr.toString();
        } catch (Exception e) {
            return "[]";
        }
    }

    private File ensureBin(String name) throws Exception {
        File dir = new File(getFilesDir(), "xray");
        dir.mkdirs();
        File dest = new File(dir, name);
        if (!dest.exists() || dest.length() < 100) {
            InputStream in = getAssets().open(name);
            FileOutputStream out = new FileOutputStream(dest);
            byte[] buf = new byte[8192];
            int n;
            while ((n = in.read(buf)) > 0) out.write(buf, 0, n);
            out.close();
            in.close();
        }
        dest.setExecutable(true);
        return dest;
    }

    private static String readAll(InputStream in) throws Exception {
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        byte[] buf = new byte[4096];
        int n;
        while ((n = in.read(buf)) > 0) out.write(buf, 0, n);
        in.close();
        return out.toString("UTF-8");
    }

    private static String jsonQuote(String s) {
        if (s == null) s = "";
        return "\"" + s.replace("\\", "\\\\").replace("\"", "\\\"").replace("\n", " ") + "\"";
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (requestCode == 77 && resultCode == RESULT_OK) {
            startVpn();
        }
    }
}
