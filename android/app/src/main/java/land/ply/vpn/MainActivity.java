package land.ply.vpn;

import android.app.Activity;
import android.content.Intent;
import android.net.VpnService;
import android.os.Build;
import android.os.Bundle;
import android.webkit.JavascriptInterface;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;

public class MainActivity extends Activity {
    public static final String EXTRA_URL = "land.ply.vpn.URL";
    public static final String EXTRA_SPLIT = "land.ply.vpn.SPLIT";

    private String pendingUrl = "";
    private boolean pendingSplit = true;

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
            runOnUiThread(new Runnable() {
                @Override public void run() {
                    pendingUrl = url == null ? "" : url;
                    pendingSplit = split;
                    Intent prep = VpnService.prepare(MainActivity.this);
                    if (prep != null) {
                        startActivityForResult(prep, 77);
                    } else {
                        startVpn(pendingUrl, pendingSplit);
                    }
                }
            });
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

    private void startVpn(String url, boolean split) {
        Intent run = new Intent(this, PlyVpnService.class);
        run.putExtra(EXTRA_URL, url);
        run.putExtra(EXTRA_SPLIT, split);
        if (Build.VERSION.SDK_INT >= 26) startForegroundService(run);
        else startService(run);
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (requestCode == 77 && resultCode == RESULT_OK) {
            startVpn(pendingUrl, pendingSplit);
        }
    }
}
