package com.swapstudio.app;

import android.app.Activity;
import android.content.ActivityNotFoundException;
import android.content.ContentValues;
import android.content.Intent;
import android.media.MediaScannerConnection;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Environment;
import android.provider.MediaStore;
import android.util.Base64;
import android.view.View;
import android.view.WindowInsets;
import android.webkit.JavascriptInterface;
import android.webkit.ValueCallback;
import android.webkit.WebChromeClient;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.FrameLayout;

import java.io.File;
import java.io.FileOutputStream;
import java.io.OutputStream;

/**
 * SwapStudio: hosts the bundled web app (assets/index.html) in a WebView.
 * Adds a photo picker for <input type="file"> and a bridge that saves results to the gallery.
 */
public class MainActivity extends Activity {

    private static final int REQ_PICK = 1001;
    private WebView web;
    private ValueCallback<Uri[]> pendingPick;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        FrameLayout root = new FrameLayout(this);
        root.setBackgroundColor(0xFF0F1115);
        web = new WebView(this);
        root.addView(web, new FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT));
        setContentView(root);

        // Keep content clear of the status and navigation bars (edge-to-edge on Android 15+).
        if (Build.VERSION.SDK_INT >= 30) {
            root.setOnApplyWindowInsetsListener(new View.OnApplyWindowInsetsListener() {
                @Override
                public WindowInsets onApplyWindowInsets(View v, WindowInsets insets) {
                    android.graphics.Insets b = insets.getInsets(
                            WindowInsets.Type.systemBars() | WindowInsets.Type.displayCutout());
                    v.setPadding(b.left, b.top, b.right, b.bottom);
                    return WindowInsets.CONSUMED;
                }
            });
        }

        WebSettings s = web.getSettings();
        s.setJavaScriptEnabled(true);
        s.setDomStorageEnabled(true);
        s.setAllowFileAccess(true);
        s.setLoadWithOverviewMode(true);
        s.setUseWideViewPort(true);

        web.setWebViewClient(new WebViewClient());
        web.setWebChromeClient(new WebChromeClient() {
            @Override
            public boolean onShowFileChooser(WebView view, ValueCallback<Uri[]> callback,
                                             FileChooserParams params) {
                if (pendingPick != null) pendingPick.onReceiveValue(null);
                pendingPick = callback;
                Intent intent = params.createIntent();
                intent.setType("image/*");
                try {
                    startActivityForResult(intent, REQ_PICK);
                } catch (ActivityNotFoundException e) {
                    pendingPick = null;
                    return false;
                }
                return true;
            }
        });
        web.addJavascriptInterface(new Bridge(), "AndroidBridge");

        if (savedInstanceState != null) {
            web.restoreState(savedInstanceState);
        } else {
            web.loadUrl("file:///android_asset/index.html");
        }
    }

    @Override
    protected void onSaveInstanceState(Bundle outState) {
        super.onSaveInstanceState(outState);
        web.saveState(outState);
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        if (requestCode == REQ_PICK) {
            if (pendingPick != null) {
                pendingPick.onReceiveValue(WebChromeClient.FileChooserParams.parseResult(resultCode, data));
                pendingPick = null;
            }
            return;
        }
        super.onActivityResult(requestCode, resultCode, data);
    }

    @Override
    public void onBackPressed() {
        if (web != null && web.canGoBack()) web.goBack();
        else super.onBackPressed();
    }

    /** Called from JavaScript as AndroidBridge.saveImage(base64Jpeg, fileName). */
    private class Bridge {
        @JavascriptInterface
        public String saveImage(String base64, String name) {
            try {
                byte[] bytes = Base64.decode(base64, Base64.DEFAULT);
                if (Build.VERSION.SDK_INT >= 29) {
                    ContentValues v = new ContentValues();
                    v.put(MediaStore.Images.Media.DISPLAY_NAME, name);
                    v.put(MediaStore.Images.Media.MIME_TYPE, "image/jpeg");
                    v.put(MediaStore.Images.Media.RELATIVE_PATH,
                            Environment.DIRECTORY_PICTURES + "/SwapStudio");
                    Uri uri = getContentResolver().insert(MediaStore.Images.Media.EXTERNAL_CONTENT_URI, v);
                    if (uri == null) return "Could not save: the gallery refused the file.";
                    OutputStream os = getContentResolver().openOutputStream(uri);
                    if (os == null) return "Could not save: no output stream.";
                    try {
                        os.write(bytes);
                    } finally {
                        os.close();
                    }
                    return "Saved to Pictures/SwapStudio in your gallery.";
                } else {
                    File dir = new File(getExternalFilesDir(Environment.DIRECTORY_PICTURES), "SwapStudio");
                    if (!dir.exists() && !dir.mkdirs()) return "Could not save: cannot create folder.";
                    File f = new File(dir, name);
                    FileOutputStream fos = new FileOutputStream(f);
                    try {
                        fos.write(bytes);
                    } finally {
                        fos.close();
                    }
                    MediaScannerConnection.scanFile(MainActivity.this,
                            new String[]{f.getAbsolutePath()}, new String[]{"image/jpeg"}, null);
                    return "Saved to " + f.getAbsolutePath();
                }
            } catch (Exception e) {
                return "Could not save: " + e.getMessage();
            }
        }
    }
}
