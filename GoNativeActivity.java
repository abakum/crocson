package org.golang.app;

import android.app.Activity;
import android.app.NativeActivity;
import android.content.Context;
import android.content.Intent;
import android.content.pm.ActivityInfo;
import android.content.pm.PackageManager;
import android.content.res.Configuration;
import android.graphics.Rect;
import android.content.ClipData;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;

import java.util.ArrayList;
import android.text.Editable;
import android.text.InputType;
import android.text.TextWatcher;
import android.text.method.DigitsKeyListener;
import android.util.Log;
import android.view.Gravity;
import android.view.KeyCharacterMap;
import android.view.View;
import android.view.WindowInsets;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.InputMethodManager;
import android.view.KeyEvent;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.TextView;
import android.widget.TextView.OnEditorActionListener;
import android.widget.Toast;

public class GoNativeActivity extends NativeActivity {
	private static GoNativeActivity goNativeActivity;
	private static final String TAG = "croc";
	private static final int FILE_OPEN_CODE = 1;
	private static final int FILE_SAVE_CODE = 2;

	private static final int DEFAULT_INPUT_TYPE = InputType.TYPE_TEXT_FLAG_NO_SUGGESTIONS;

	private static final int DEFAULT_KEYBOARD_CODE = 0;
	private static final int SINGLELINE_KEYBOARD_CODE = 1;
	private static final int NUMBER_KEYBOARD_CODE = 2;
	private static final int PASSWORD_KEYBOARD_CODE = 3;

    private native void filePickerReturned(String str);
    private native void insetsChanged(int top, int bottom, int left, int right);
    private native void keyboardTyped(String str);
    private native void keyboardDelete();
    private native void backPressed();
    private native void setDarkMode(boolean dark);
    private native void lifecycleEvent(String event);
    private native void intentURI(String uri);

    private native void intentText(String text);

    private void logIntentURI(String uri) {
        Log.d(TAG, "Java: intentURI sending to Go: " + uri);
        intentURI(uri);
    }

    private void logIntentText(String text) {
        Log.d(TAG, "Java: intentText sending to Go: " + (text != null && text.length() > 50 ? text.substring(0, 50) + "..." : text));
        intentText(text);
    }
	private EditText mTextEdit;
	private boolean ignoreKey = false;
	private boolean keyboardUp = false;
	private ArrayList<String> pendingIntentURIs = null;

	public GoNativeActivity() {
		super();
		goNativeActivity = this;
	}

	String getTmpdir() {
		return getCacheDir().getAbsolutePath();
	}

	void updateLayout() {
	    try {
            WindowInsets insets = getWindow().getDecorView().getRootWindowInsets();
            if (insets == null) {
                return;
            }

            insetsChanged(insets.getSystemWindowInsetTop(), insets.getSystemWindowInsetBottom(),
                insets.getSystemWindowInsetLeft(), insets.getSystemWindowInsetRight());
        } catch (java.lang.NoSuchMethodError e) {
    	    Rect insets = new Rect();
            getWindow().getDecorView().getWindowVisibleDisplayFrame(insets);

            View view = findViewById(android.R.id.content).getRootView();
            insetsChanged(insets.top, view.getHeight() - insets.height() - insets.top,
                insets.left, view.getWidth() - insets.width() - insets.left);
        }
    }

    static void showKeyboard(int keyboardType) {
        goNativeActivity.doShowKeyboard(keyboardType);
        goNativeActivity.keyboardUp = true;
    }

    void doShowKeyboard(final int keyboardType) {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                int imeOptions = EditorInfo.IME_FLAG_NO_ENTER_ACTION;
                int inputType = DEFAULT_INPUT_TYPE;
                String keys = "";
                switch (keyboardType) {
                    case DEFAULT_KEYBOARD_CODE:
                        imeOptions = EditorInfo.IME_FLAG_NO_ENTER_ACTION;
                        break;
                    case SINGLELINE_KEYBOARD_CODE:
                        imeOptions = EditorInfo.IME_ACTION_DONE;
                        break;
                    case NUMBER_KEYBOARD_CODE:
                        imeOptions = EditorInfo.IME_ACTION_DONE;
                        inputType |= InputType.TYPE_CLASS_NUMBER | InputType.TYPE_NUMBER_VARIATION_NORMAL;
                        keys = "0123456789.,-' "; // work around android bug where some number keys are blocked
                        break;
                    case PASSWORD_KEYBOARD_CODE:
                        imeOptions = EditorInfo.IME_ACTION_DONE;
                        inputType |= InputType.TYPE_TEXT_VARIATION_PASSWORD;
                    default:
                        Log.e("Fyne", "unknown keyboard type, use default");
                }
                mTextEdit.setImeOptions(imeOptions|EditorInfo.IME_FLAG_NO_FULLSCREEN);
                mTextEdit.setInputType(inputType);
                if (keys != "") {
                    mTextEdit.setKeyListener(DigitsKeyListener.getInstance(keys));
                }

                mTextEdit.setOnEditorActionListener(new OnEditorActionListener() {
                    @Override
                    public boolean onEditorAction(TextView v, int actionId, KeyEvent event) {
                        if (actionId == EditorInfo.IME_ACTION_DONE) {
                            keyboardTyped("\n");
                        }
                        return false;
                    }
                });

                // always place one character so all keyboards can send backspace
                ignoreKey = true;
                mTextEdit.setText(" ");
                mTextEdit.setSelection(mTextEdit.getText().length());
                ignoreKey = false;

                mTextEdit.setVisibility(View.VISIBLE);
                mTextEdit.bringToFront();
                mTextEdit.requestFocus();

                InputMethodManager m = (InputMethodManager) getSystemService(Context.INPUT_METHOD_SERVICE);
                m.showSoftInput(mTextEdit, 0);
            }
        });
    }

    static void hideKeyboard() {
        goNativeActivity.doHideKeyboard();
        goNativeActivity.keyboardUp = false;
    }

    static void startCrocsonService() {
        try {
            Class<?> serviceClass = Class.forName("com.github.abakum.crocson.CrocsonService");
            Intent intent = new Intent(goNativeActivity, serviceClass);
            if (Build.VERSION.SDK_INT >= 26) {
                goNativeActivity.startForegroundService(intent);
            } else {
                goNativeActivity.startService(intent);
            }
            Log.d(TAG, "Java: Foreground service started (API " + Build.VERSION.SDK_INT + ")");
        } catch (Exception e) {
            Log.e(TAG, "Java: startCrocsonService failed: " + e.getMessage());
        }
    }

    static void stopCrocsonService() {
        try {
            Class<?> serviceClass = Class.forName("com.github.abakum.crocson.CrocsonService");
            Intent intent = new Intent(goNativeActivity, serviceClass);
            goNativeActivity.stopService(intent);
            Log.d(TAG, "Java: Foreground service stopped");
        } catch (Exception e) {
            Log.e(TAG, "Java: stopCrocsonService failed: " + e.getMessage());
        }
    }

    static void showToast(String message) {
        goNativeActivity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                Toast.makeText(goNativeActivity, message, Toast.LENGTH_SHORT).show();
            }
        });
        Log.d(TAG, "Java: showToast: " + message);
    }

    static int getApiLevel() {
        return Build.VERSION.SDK_INT;
    }

    static String getFileName(String uriStr) {
        try {
            android.net.Uri uri = android.net.Uri.parse(uriStr);
            String[] projection = {android.provider.OpenableColumns.DISPLAY_NAME};
            android.database.Cursor cursor = goNativeActivity.getContentResolver().query(uri, projection, null, null, null);
            if (cursor != null) {
                try {
                    if (cursor.moveToFirst()) {
                        return cursor.getString(0);
                    }
                } finally {
                    cursor.close();
                }
            }
        } catch (Exception e) {
            Log.e(TAG, "Java: getFileName failed: " + e.getMessage());
        }
        return null;
    }

    static boolean canListDirectory(String uriStr) {
        try {
            android.net.Uri uri = android.net.Uri.parse(uriStr);
            if (Build.VERSION.SDK_INT >= 21) {
                android.net.Uri childUri = android.provider.DocumentsContract.buildChildDocumentsUriUsingTree(uri, null);
                if (childUri != null) return true;
            }
            String[] projection = {"document_id"};
            android.database.Cursor cursor = goNativeActivity.getContentResolver().query(uri, projection, null, null, null);
            if (cursor != null) {
                cursor.close();
                return true;
            }
        } catch (Exception e) {
            Log.e(TAG, "Java: canListDirectory failed: " + e.getMessage());
        }
        return false;
    }

    static void openIntent(String intentStr) {
        try {
            android.content.Intent intent = android.content.Intent.parseUri(intentStr, android.content.Intent.URI_INTENT_SCHEME);
            goNativeActivity.startActivity(intent);
        } catch (Exception e) {
            Log.e(TAG, "Java: openIntent failed: " + e.getMessage());
        }
    }

    static String getMimeType(String uriStr) {
        try {
            android.net.Uri uri = android.net.Uri.parse(uriStr);
            String type = goNativeActivity.getContentResolver().getType(uri);
            if (type != null) return type;
            String[] projection = {"mime_type"};
            android.database.Cursor cursor = goNativeActivity.getContentResolver().query(uri, projection, null, null, null);
            if (cursor != null) {
                try {
                    if (cursor.moveToFirst()) {
                        return cursor.getString(0);
                    }
                } finally {
                    cursor.close();
                }
            }
        } catch (Exception e) {
            Log.e(TAG, "Java: getMimeType failed: " + e.getMessage());
        }
        return null;
    }

    void doHideKeyboard() {
        InputMethodManager imm = (InputMethodManager) getSystemService(Context.INPUT_METHOD_SERVICE);
        View view = findViewById(android.R.id.content).getRootView();
        imm.hideSoftInputFromWindow(view.getWindowToken(), 0);

        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                mTextEdit.setVisibility(View.GONE);
            }
        });
    }

    static void showFileOpen(String mimes) {
        goNativeActivity.doShowFileOpen(mimes);
    }

    void doShowFileOpen(String mimes) {
        Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
        if ("application/x-directory".equals(mimes) && Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
            intent = new Intent(Intent.ACTION_OPEN_DOCUMENT_TREE); // ask for a directory picker if OS supports it
            intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
        } else if (mimes.contains("|") && Build.VERSION.SDK_INT >= Build.VERSION_CODES.KITKAT) {
            intent.setType("*/*");
            intent.putExtra(Intent.EXTRA_MIME_TYPES, mimes.split("\\|"));
            intent.addCategory(Intent.CATEGORY_OPENABLE);
        } else {
            intent.setType(mimes);
            intent.addCategory(Intent.CATEGORY_OPENABLE);
        }
        startActivityForResult(Intent.createChooser(intent, "Open File"), FILE_OPEN_CODE);
    }

    static void showFileSave(String mimes, String filename) {
        goNativeActivity.doShowFileSave(mimes, filename);
    }

    void doShowFileSave(String mimes, String filename) {
        Intent intent = new Intent(Intent.ACTION_CREATE_DOCUMENT);
        if (mimes.contains("|") && Build.VERSION.SDK_INT >= Build.VERSION_CODES.KITKAT) {
            intent.setType("*/*");
            intent.putExtra(Intent.EXTRA_MIME_TYPES, mimes.split("\\|"));
        } else {
            intent.setType(mimes);
        }
        intent.putExtra(Intent.EXTRA_TITLE, filename);
        intent.addCategory(Intent.CATEGORY_OPENABLE);
        startActivityForResult(Intent.createChooser(intent, "Save File"), FILE_SAVE_CODE);
    }
	static int getRune(int deviceId, int keyCode, int metaState) {
		try {
			int rune = KeyCharacterMap.load(deviceId).get(keyCode, metaState);
			if (rune == 0) {
				return -1;
			}
			return rune;
		} catch (KeyCharacterMap.UnavailableException e) {
			return -1;
		} catch (Exception e) {
			Log.e("Fyne", "exception reading KeyCharacterMap", e);
			return -1;
		}
	}

	private void load() {
		// Interestingly, NativeActivity uses a different method
		// to find native code to execute, avoiding
		// System.loadLibrary. The result is Java methods
		// implemented in C with JNIEXPORT (and JNI_OnLoad) are not
		// available unless an explicit call to System.loadLibrary
		// is done. So we do it here, borrowing the name of the
		// library from the same AndroidManifest.xml metadata used
		// by NativeActivity.
		try {
			ActivityInfo ai = getPackageManager().getActivityInfo(
					getIntent().getComponent(), PackageManager.GET_META_DATA);
			if (ai.metaData == null) {
				Log.e("Fyne", "loadLibrary: no manifest metadata found");
				return;
			}
			String libName = ai.metaData.getString("android.app.lib_name");
			System.loadLibrary(libName);
		} catch (Exception e) {
			Log.e("Fyne", "loadLibrary android.app.lib_name failed", e);
		}
	}

	@Override
	public void onCreate(Bundle savedInstanceState) {
		load();
		super.onCreate(savedInstanceState);
		setupEntry();
		updateTheme(getResources().getConfiguration());
		Log.d(TAG, "Java: onCreate");
		lifecycleEvent("create");

		Intent intent = getIntent();
		int flags = (intent != null) ? intent.getFlags() : 0;
		boolean fromHistory = (flags & Intent.FLAG_ACTIVITY_LAUNCHED_FROM_HISTORY) != 0;
		boolean broughtToFront = (flags & Intent.FLAG_ACTIVITY_BROUGHT_TO_FRONT) != 0;

		Log.d(TAG, "Java: onCreate flags=" + flags + ", LAUNCHED_FROM_HISTORY=" + fromHistory +
		      ", BROUGHT_TO_FRONT=" + broughtToFront + ", savedInstanceState=" +
		      (savedInstanceState == null ? "null" : "not null"));

		if (savedInstanceState == null) {
			if (!fromHistory) {
				// Fresh launch from launcher or file manager
				Log.d(TAG, "Java: onCreate processing intent (fresh launch)");
				processIntentData(intent);
			} else {
				// Returning from Recents → skip processing (duplicate)
				Log.d(TAG, "Java: onCreate skipping intent (LAUNCHED_FROM_HISTORY=true)");
			}
		} else {
			Log.d(TAG, "Java: onCreate skipping intent (savedInstanceState != null, config change)");
		}

		View view = findViewById(android.R.id.content).getRootView();
		view.addOnLayoutChangeListener(new View.OnLayoutChangeListener() {
			public void onLayoutChange (View v, int left, int top, int right, int bottom,
			                            int oldLeft, int oldTop, int oldRight, int oldBottom) {
				GoNativeActivity.this.updateLayout();
			}
		});
    }

    private void setupEntry() {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                mTextEdit = new EditText(goNativeActivity);
                mTextEdit.setVisibility(View.GONE);
                mTextEdit.setInputType(DEFAULT_INPUT_TYPE);

                FrameLayout.LayoutParams mEditTextLayoutParams = new FrameLayout.LayoutParams(
                    FrameLayout.LayoutParams.WRAP_CONTENT, FrameLayout.LayoutParams.WRAP_CONTENT);
                mTextEdit.setLayoutParams(mEditTextLayoutParams);
                addContentView(mTextEdit, mEditTextLayoutParams);

                // always place one character so all keyboards can send backspace
                mTextEdit.setText(" ");
                mTextEdit.setSelection(mTextEdit.getText().length());

                mTextEdit.addTextChangedListener(new TextWatcher() {
                    @Override
                    public void onTextChanged(CharSequence s, int start, int before, int count) {
                        if (ignoreKey) {
                            return;
                        }
                        if (count > 0) {
                            keyboardTyped(s.subSequence(start,start+count).toString());
                        }
                    }

                    @Override
                    public void beforeTextChanged(CharSequence s, int start, int count, int after) {
                        if (ignoreKey) {
                            return;
                        }
                        if (count > 0) {
                            for (int i = 0; i < count; i++) {
                                // send a backspace
                                keyboardDelete();
                            }
                        }
                    }

                    @Override
                    public void afterTextChanged(Editable s) {
                        // always place one character so all keyboards can send backspace
                        if (s.length() < 1) {
                            ignoreKey = true;
                            mTextEdit.setText(" ");
                            mTextEdit.setSelection(mTextEdit.getText().length());
                            ignoreKey = false;
                            return;
                        }
                    }
                });
            }
        });
	}

	@Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        // unhandled request
        if (requestCode != FILE_OPEN_CODE && requestCode != FILE_SAVE_CODE) {
            return;
        }

        // dialog was cancelled
        if (resultCode != Activity.RESULT_OK) {
            filePickerReturned("");
            return;
        }

        Uri uri = data.getData();
        filePickerReturned(uri.toString());
    }

    @Override
    public void onBackPressed() {
        if (goNativeActivity.keyboardUp) {
            hideKeyboard();
            return;
        }

        // skip the default behaviour - we can call finishActivity if we want to go back
        backPressed();
    }

    public void finishActivity() {
        runOnUiThread(new Runnable() {
            @Override
            public void run() {
                GoNativeActivity.super.onBackPressed();
            }
        });
    }

    @Override
    public void onConfigurationChanged(Configuration config) {
        super.onConfigurationChanged(config);
        updateTheme(config);
    }

    protected void updateTheme(Configuration config) {
        boolean dark = (config.uiMode & Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES;
        setDarkMode(dark);
    }

    @Override
    protected void onStart() {
        super.onStart();
        Log.d(TAG, "Java: onStart");
        lifecycleEvent("start");
    }

    @Override
    protected void onUserLeaveHint() {
        super.onUserLeaveHint();
        Log.d(TAG, "Java: onUserLeaveHint");
        if (Build.VERSION.SDK_INT <= 28) {
            finishActivity();
        }
    }

    @Override
    protected void onRestart() {
        super.onRestart();
        Log.d(TAG, "Java: onRestart");
        lifecycleEvent("restart");
    }

    @Override
    protected void onResume() {
        super.onResume();
        Log.d(TAG, "Java: onResume");
        lifecycleEvent("resume");
    }

    @Override
    protected void onPause() {
        Log.d(TAG, "Java: onPause");
        lifecycleEvent("pause");
        super.onPause();
    }

    @Override
    protected void onStop() {
        Log.d(TAG, "Java: onStop");
        lifecycleEvent("stop");
        super.onStop();
    }

    @Override
    protected void onDestroy() {
        Log.d(TAG, "Java: onDestroy");
        lifecycleEvent("destroy");
        super.onDestroy();
    }

    @Override
    protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        setIntent(intent);
        Log.d(TAG, "Java: onNewIntent action=" + (intent != null ? intent.getAction() : "null"));
        processIntentData(intent);
    }

    private void sendIntentURIs(ArrayList<String> uriList) {
        Log.d(TAG, "Java: sendIntentURIs called with " + uriList.size() + " URIs");
        boolean needsPermission = false;
        for (String uri : uriList) {
            if (uri.startsWith("file://")) {
                needsPermission = true;
                Log.d(TAG, "Java: sendIntentURIs needs permission for file:// URI");
                break;
            }
        }
        if (needsPermission && checkSelfPermission("android.permission.READ_EXTERNAL_STORAGE") != 0) {
            Log.d(TAG, "Java: sendIntentURIs requesting permissions");
            pendingIntentURIs = uriList;
            lifecycleEvent("permissionDialog");
            requestPermissions(new String[]{
                "android.permission.READ_EXTERNAL_STORAGE",
                "android.permission.WRITE_EXTERNAL_STORAGE"
            }, 123);
            return;
        }
        Log.d(TAG, "Java: sendIntentURIs sending " + uriList.size() + " URIs to Go");
        for (String uri : uriList) {
            logIntentURI(uri);
        }
    }

    @Override
    public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] grantResults) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults);
        Log.d(TAG, "Java: onRequestPermissionsResult requestCode=" + requestCode + ", grantResults=" + grantResults[0]);
        if (requestCode == 123 && pendingIntentURIs != null) {
            boolean granted = true;
            for (int result : grantResults) {
                if (result != 0) granted = false;
            }
            Log.d(TAG, "Java: permissionResult granted=" + granted + ", pending URIs=" + pendingIntentURIs.size());
            if (granted) {
                for (String uri : pendingIntentURIs) {
                    Log.d(TAG, "Java: permissionResult sending URI=" + uri);
                    logIntentURI(uri);
                }
            }
            pendingIntentURIs = null;
        }
    }

    private void processIntentData(Intent intent) {
        if (intent == null || intent.getAction() == null || Intent.ACTION_MAIN.equals(intent.getAction())) {
            Log.d(TAG, "Java: processIntentData skipped: null, null action, or MAIN");
            return;
        }

        String action = intent.getAction();
        String type = intent.getType();
        ArrayList<String> uriList = new ArrayList<>();

        Log.d(TAG, "Java: processIntentData action=" + action + ", type=" + type);

        ClipData clipData = intent.getClipData();
        if (clipData != null) {
            for (int i = 0; i < clipData.getItemCount(); i++) {
                ClipData.Item item = clipData.getItemAt(i);
                if (item.getUri() != null) {
                    uriList.add(item.getUri().toString());
                    Log.d(TAG, "Java: processIntentData ClipData uri=" + item.getUri().toString());
                }
                if (item.getText() != null) {
                    logIntentText(item.getText().toString());
                }
            }
            if (!uriList.isEmpty()) {
                Log.d(TAG, "Java: processIntentData sending " + uriList.size() + " URIs");
                sendIntentURIs(uriList);
            }
            return;
        }

        if (Intent.ACTION_SEND.equals(action) && "text/plain".equals(type)) {
            String text = intent.getStringExtra(Intent.EXTRA_TEXT);
            if (text != null) {
                Log.d(TAG, "Java: processIntentData sending text/plain");
                logIntentText(text);
                return;
            }
        }

        if (Intent.ACTION_VIEW.equals(action)) {
            Uri uri = intent.getData();
            if (uri != null) {
                uriList.add(uri.toString());
                Log.d(TAG, "Java: processIntentData sending VIEW uri=" + uri.toString());
                sendIntentURIs(uriList);
                return;
            }
        }

        if (Intent.ACTION_SEND.equals(action)) {
            Uri stream = intent.getParcelableExtra(Intent.EXTRA_STREAM);
            if (stream != null) {
                uriList.add(stream.toString());
                Log.d(TAG, "Java: processIntentData sending SEND stream=" + stream.toString());
                sendIntentURIs(uriList);
                return;
            }
        }

        if (Intent.ACTION_SEND_MULTIPLE.equals(action)) {
            ArrayList<Uri> uris = intent.getParcelableArrayListExtra(Intent.EXTRA_STREAM);
            if (uris != null && !uris.isEmpty()) {
                for (Uri u : uris) {
                    uriList.add(u.toString());
                }
                Log.d(TAG, "Java: processIntentData sending SEND_MULTIPLE (" + uris.size() + " URIs)");
                sendIntentURIs(uriList);
                return;
            }
        }
    }
}
