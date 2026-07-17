package org.golang.app;

import android.app.Activity;
import android.app.Dialog;
import android.app.NativeActivity;
import android.content.Context;
import android.content.DialogInterface;
import android.content.Intent;
import android.content.pm.ActivityInfo;
import android.content.pm.PackageManager;
import android.content.res.Configuration;
import android.graphics.Color;
import android.graphics.ImageFormat;
import android.graphics.Rect;
import android.content.ClipData;
import android.hardware.Camera;
import android.net.Uri;
import android.net.wifi.WifiManager;
import android.os.Build;
import android.os.Bundle;
import android.os.Handler;
import android.os.HandlerThread;
import android.os.SystemClock;

import java.util.ArrayList;
import java.util.List;
import android.text.Editable;
import android.text.InputType;
import android.text.TextWatcher;
import android.text.method.DigitsKeyListener;
import android.util.Log;
import android.view.Gravity;
import android.view.KeyCharacterMap;
import android.view.SurfaceHolder;
import android.view.SurfaceView;
import android.view.View;
import android.view.ViewGroup;
import android.view.Window;
import android.view.WindowManager;
import android.view.WindowInsets;
import android.view.WindowInsetsController;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.InputMethodManager;
import android.view.KeyEvent;
import android.widget.Button;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.TextView;
import android.widget.TextView.OnEditorActionListener;
import android.widget.Toast;

import android.graphics.Canvas;
import android.graphics.Paint;


public class GoNativeActivity extends NativeActivity {
	private static GoNativeActivity goNativeActivity;
	private static final String TAG = "croc";
	private static final int FILE_OPEN_CODE = 1;
	private static final int FILE_SAVE_CODE = 2;
	private static final int INTENT_OPEN_CODE = 3;

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

    // Built-in QR scanner: feed one NV21 preview frame to Go.
    // Returns true to keep streaming (Go re-adds the buffer),
    // false to stop (decode hit / cancel / closed / error).
    private native boolean cameraFrame(byte[] data, int w, int h);

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

    private static WifiManager.MulticastLock multicastLock;

    static boolean acquireMulticastLock() {
        try {
            if (multicastLock != null && multicastLock.isHeld()) {
                return true;
            }
            if (goNativeActivity == null) {
                return false;
            }
            WifiManager wm = (WifiManager) goNativeActivity.getSystemService(Context.WIFI_SERVICE);
            if (wm == null) {
                return false;
            }
            multicastLock = wm.createMulticastLock("croc");
            multicastLock.setReferenceCounted(false);
            multicastLock.acquire();
            return true;
        } catch (Throwable t) {
            Log.e(TAG, "acquireMulticastLock failed", t);
            return false;
        }
    }

    static boolean releaseMulticastLock() {
        try {
            if (multicastLock != null && multicastLock.isHeld()) {
                multicastLock.release();
            }
            return true;
        } catch (Throwable t) {
            Log.e(TAG, "releaseMulticastLock failed", t);
            return false;
        }
    }

    // ----------------------------------------------------------------------
    // Built-in QR scanner camera (Camera1). Deprecated but framework-only:
    // fyne's android build compiles project .java against android.jar only,
    // so CameraX/AndroidX cannot be used.
    // ----------------------------------------------------------------------

    private static final int CAMERA_PERMISSION_CODE = 201;

    private static Camera qrCamera = null;
    private static volatile boolean qrCameraRunning = false;
    // Состояние фонарика QR-камеры. Инвертируется тапом по оверлею превью;
    // применяется на камерном потоке как FLASH_MODE_TORCH/OFF. Сбрасывается в OFF
    // при остановке камеры (один переключатель на живую сессию).
    private static volatile boolean qrFlashOn = false;
    // Задняя камера поддерживает FLASH_MODE_TORCH (запрос один раз в startCameraWithHolder).
    // Гарант для toggleCameraFlash (молчаливый no-op если false).
    private static volatile boolean qrFlashTorchSupported = false;
    private static int qrPreviewWidth = 0;
    private static int qrPreviewHeight = 0;
    private static int qrFrameCount = 0;
    // Reused buffer holding the centered square of the Y plane, pre-cropped on the
    // camera thread so Go only receives the square Y (drops chroma, halves JNI
    // traffic). Sized to min(previewW, previewH)^2; rotation-invariant (the
    // centered square of the raw frame is the same at any grip).
    private static byte[] qrSquareBuf = null;
    private static int qrSquareSide = 0;

    // Dedicated camera thread: Camera.open()/config/startPreview/release must
    // never run on the UI or GL thread (blocking -> ANR / visual freeze). All
    // camera hardware ops happen on this HandlerThread's Handler.
    private static HandlerThread qrCameraThread = null;
    private static Handler qrCameraHandler = null;
    // Native full-screen Dialog hosting a SurfaceView color preview. The camera
    // is given the SurfaceView's Surface (setPreviewDisplay) — a real, consumed
    // native surface (separate window, like Fyne's own GLSurfaceView) instead of
    // a dummy unconsumed SurfaceTexture: that is what keeps the capture pipeline
    // from stalling on old Camera1->Camera2 HALs (Android 9/10 freeze). SurfaceView
    // is used (not TextureView) because a TextureView surface never materializes
    // in this GL/Fyne NativeActivity (black preview on emulator + device).
    private static Dialog qrDialog = null;
    private static SurfaceView qrSurface = null;
    // True while the camera Dialog is up & the camera is running. Guards the
    // lifecycle "pause" dismiss (skips the first-run permission-request pause,
    // which happens before any camera Dialog is shown).
    private static volatile boolean qrDialogShown = false;
    // Back-camera sensor orientation (degrees), cached at camera open so
    // reapplyPreviewOrientation can recompute the display angle on rotation
    // without re-querying CameraInfo.
    private static int qrSensorOrientJava = 90;
    // Decode-feed throttle: preview is rendered natively at full rate, so Go only
    // needs a few fps for QR decode. Timestamp (uptimeMillis) of the last frame
    // forwarded to Go; frames arriving sooner than 100 ms are dropped here.
    private static long qrLastDecodeFeedMs = 0;
    // Preview-size bounds {largeBound, smallBound} derived from the physical display
    // edges minus total system insets, captured on the UI thread in showCameraDialog
    // and read by startCameraWithHolder on the camera thread (so the camera thread
    // never touches Display/WindowInsets). Orientation-independent -> the chosen
    // preview fits in BOTH portrait and landscape (no overflow after rotation).
    private static volatile int qrBoundLarge = 0;
    private static volatile int qrBoundSmall = 0;

    private static final Camera.PreviewCallback qrPreviewCallback = new Camera.PreviewCallback() {
        @Override
        public void onPreviewFrame(byte[] data, Camera camera) {
            if (!qrCameraRunning || data == null || camera == null) {
                return;
            }
            int n = ++qrFrameCount;
            if (n == 1 || n % 30 == 0) {
                Log.d(TAG, "Java: onPreviewFrame #" + n + " " + qrPreviewWidth + "x" + qrPreviewHeight);
            }
            // Throttle decode feed to ~10 fps: preview is a native color surface
            // now, Go only needs a few frames/sec for QR decode. This cuts JNI
            // traffic + Go GC pressure ~3x while keeping the visible preview full
            // rate. When throttled, just re-add the buffer and keep streaming.
            boolean keep = true;
            if (SystemClock.uptimeMillis() - qrLastDecodeFeedMs >= 100) {
                qrLastDecodeFeedMs = SystemClock.uptimeMillis();
                try {
                    if (goNativeActivity != null) {
                        keep = goNativeActivity.feedSquareFrame(data);
                    }
                } catch (Throwable t) {
                    Log.e(TAG, "Java: cameraFrame threw: " + t.getMessage());
                    keep = false;
                }
            }
            if (keep && qrCameraRunning) {
                camera.addCallbackBuffer(data);
            } else {
                // stop on Go's request; release off the camera callback thread
                // (dismissCameraDialog posts stop to the camera HandlerThread).
                qrCameraRunning = false;
                dismissCameraDialog();
            }
        }
    };

    static boolean hasCameraPermission() {
        try {
            if (goNativeActivity == null) return false;
            return goNativeActivity.checkSelfPermission("android.permission.CAMERA") == PackageManager.PERMISSION_GRANTED;
        } catch (Throwable t) {
            Log.e(TAG, "Java: hasCameraPermission failed: " + t.getMessage());
            return false;
        }
    }

    static boolean requestCameraPermission() {
        try {
            if (goNativeActivity == null) return false;
            goNativeActivity.requestPermissions(new String[]{"android.permission.CAMERA"}, CAMERA_PERMISSION_CODE);
            return true;
        } catch (Throwable t) {
            Log.e(TAG, "Java: requestCameraPermission failed: " + t.getMessage());
            return false;
        }
    }

    private static int findBackCameraId() {
        int numberOfCameras = Camera.getNumberOfCameras();
        Camera.CameraInfo info = new Camera.CameraInfo();
        for (int i = 0; i < numberOfCameras; i++) {
            Camera.getCameraInfo(i, info);
            if (info.facing == Camera.CameraInfo.CAMERA_FACING_BACK) {
                return i;
            }
        }
        return numberOfCameras > 0 ? 0 : -1;
    }
  
    // Sensor orientation (degrees, CCW) of the back camera. Used by Go to rotate
    // the NV21 Y plane: setDisplayOrientation only affects SurfaceView/SurfaceTexture
    // output, NOT the raw preview bytes delivered to onPreviewFrame, so the buffer
    // always arrives in the sensor's native (landscape) orientation.
    static int getCameraSensorOrientation() {
        try {
            int id = findBackCameraId();
            if (id < 0) return -1;
            Camera.CameraInfo info = new Camera.CameraInfo();
            Camera.getCameraInfo(id, info);
            return info.orientation;
        } catch (Throwable t) {
            Log.e(TAG, "Java: getCameraSensorOrientation failed: " + t.getMessage());
            return -1;
        }
    }

    // Current display rotation in degrees (0/90/180/270), relative to the
    // device's natural orientation; -1 if unavailable. Used by Go to rotate the
    // NV21 Y plane so the QR preview/decode matches the screen orientation.
    static int getDeviceRotation() {
        try {
            if (goNativeActivity == null) return -1;
            int r = goNativeActivity.getWindowManager().getDefaultDisplay().getRotation();
            if (r == android.view.Surface.ROTATION_0) return 0;
            if (r == android.view.Surface.ROTATION_90) return 90;
            if (r == android.view.Surface.ROTATION_180) return 180;
            if (r == android.view.Surface.ROTATION_270) return 270;
            return -1;
        } catch (Throwable t) {
            Log.e(TAG, "Java: getDeviceRotation failed: " + t.getMessage());
            return -1;
        }
    }

    // Usable screen size (px) minus current system insets (status bar / nav bar).
    // UI-thread only. Falls back to raw getSize() when insets are unavailable.
    private static int[] getUsableScreenSize() {
        int screenW = 640;
        int screenH = 480;
        try {
            android.graphics.Point p = new android.graphics.Point();
            goNativeActivity.getWindowManager().getDefaultDisplay().getSize(p);
            screenW = p.x;
            screenH = p.y;
            WindowInsets insets = goNativeActivity.getWindow().getDecorView().getRootWindowInsets();
            if (insets != null) {
                screenW -= (insets.getSystemWindowInsetLeft() + insets.getSystemWindowInsetRight());
                screenH -= (insets.getSystemWindowInsetTop() + insets.getSystemWindowInsetBottom());
            }
        } catch (Throwable ignored) {}
        if (screenW < 1) screenW = 1;
        if (screenH < 1) screenH = 1;
        return new int[]{screenW, screenH};
    }

    // Orientation-independent bounds {largeBound, smallBound} for camera preview dims,
    // from the physical display edges minus total system insets. UI-thread only.
    // Using physical edges (not current-orientation W/H) makes a preview chosen here
    // fit in BOTH portrait and landscape, so it cannot overflow after rotation:
    // shortEdge - sumInsets <= usable small side in any orientation (sumInsets >= the
    // bars on the short edge in any orientation). Conservative on some devices but
    // never overflows.
    private static int[] getPreviewBounds() {
        int realW = 1080, realH = 1920;
        int sum = 0;
        try {
            android.graphics.Point p = new android.graphics.Point();
            goNativeActivity.getWindowManager().getDefaultDisplay().getRealSize(p);
            realW = p.x;
            realH = p.y;
            WindowInsets insets = goNativeActivity.getWindow().getDecorView().getRootWindowInsets();
            if (insets != null) {
                sum = (insets.getSystemWindowInsetTop() + insets.getSystemWindowInsetBottom()
                        + insets.getSystemWindowInsetLeft() + insets.getSystemWindowInsetRight());
            }
        } catch (Throwable ignored) {}
        int shortEdge = Math.min(realW, realH);
        int longEdge = Math.max(realW, realH);
        int smallBound = Math.max(1, shortEdge - sum);
        int largeBound = Math.max(1, longEdge - sum);
        return new int[]{largeBound, smallBound};
    }

    // Largest-area preview whose sides fit largeBound x smallBound (orientation-
    // independent physical-edge bounds); falls back to the smallest-area preview if
    // none fits. null if no sizes.
    private static Camera.Size choosePreviewSize(List<Camera.Size> sizes, int largeBound, int smallBound) {
        if (sizes == null || sizes.isEmpty()) return null;
        Camera.Size best = null;
        Camera.Size smallest = sizes.get(0);
        for (Camera.Size s : sizes) {
            int area = s.width * s.height;
            if (area < smallest.width * smallest.height) smallest = s;
            if (Math.max(s.width, s.height) <= largeBound && Math.min(s.width, s.height) <= smallBound) {
                if (best == null || area > best.width * best.height) best = s;
            }
        }
        return best != null ? best : smallest;
    }

    // Dialog W/H for a camera resolution, swapping dimensions in portrait so the
    // landscape-sensor preview matches the upright screen orientation.
    private static int[] computeDialogSize(int screenW, int screenH, int camW, int camH) {
        if (screenW > screenH) return new int[]{camW, camH};
        return new int[]{camH, camW};
    }


    // Lazily start the dedicated camera HandlerThread. Camera.open()/config/
    // startPreview/release must run off the UI and GL threads (blocking there
    // -> ANR / visual freeze, which was part of the Android 9/10 hang).
   private static void ensureCameraThread() {
        if (qrCameraThread == null) {
            qrCameraThread = new HandlerThread("qrCamera");
            qrCameraThread.start();
            qrCameraHandler = new Handler(qrCameraThread.getLooper());
        }
    }

    // Кастомный оверлей для отрисовки затенения и рамки квадрата
    static class QrOverlayView extends View {
        private final Paint shadowPaint;
        private final Paint windowPaint;

        public QrOverlayView(Context context) {
            super(context);
            
            setWillNotDraw(false);
            setBackgroundColor(Color.TRANSPARENT);
             if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                setLayerType(View.LAYER_TYPE_HARDWARE, null);
            }

            shadowPaint = new Paint();
            shadowPaint.setColor(Color.parseColor("#66000000")); // 40% черный
            shadowPaint.setStyle(Paint.Style.FILL);

            windowPaint = new Paint();
            windowPaint.setAntiAlias(true);
            windowPaint.setColor(Color.parseColor("#40FFFFFF")); // 25% белый
            windowPaint.setStyle(Paint.Style.STROKE);
            windowPaint.setStrokeWidth(4);
        }

        // Этот метод вызывается Android автоматически при любом изменении размеров экрана/повороте
        @Override
        protected void onSizeChanged(int w, int h, int oldw, int oldh) {
            super.onSizeChanged(w, h, oldw, oldh);
            invalidate(); // Принудительно заставляем View вызвать onDraw с новыми размерами
        }

        @Override
        protected void onDraw(Canvas canvas) {
            super.onDraw(canvas);

            int width = getWidth();
            int height = getHeight();

            // Если размеры еще не инициализированы, пропускаем шаг
            if (width == 0 || height == 0) return;

            if (width < height) {
                // 1. ПОРТРЕТ: Окно во всю ширину. Затеняем только верх и низ.
                float top = (height - width) / 2f;
                float bottom = top + width;

                canvas.drawRect(0, 0, width, top, shadowPaint);
                canvas.drawRect(0, bottom, width, height, shadowPaint);
                canvas.drawRect(0, top, width, bottom, windowPaint);
            } else {
                // 2. ЛАНДШАФТ: Окно во всю высоту. Затеняем только бока.
                float left = (width - height) / 2f;
                float right = left + height;

                canvas.drawRect(0, 0, left, height, shadowPaint);
                canvas.drawRect(right, 0, width, height, shadowPaint);
                canvas.drawRect(left, 0, right, height, windowPaint);
            }
        }
    }

    static void showCameraDialog() {
        ensureCameraThread();
        if (goNativeActivity == null) return;
        goNativeActivity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                if (qrDialog != null) { 
                    qrDialogShown = true; 
                    Log.d(TAG, "Java: showCameraDialog already shown, skipping");
                    return; 
                }
                final Activity act = goNativeActivity;
                try {
                    int[] usable = getUsableScreenSize();
                    int screenW = usable[0];
                    int screenH = usable[1];
                    int[] bounds = getPreviewBounds();
                    int largeBound = bounds[0];
                    int smallBound = bounds[1];
                    qrBoundLarge = largeBound;
                    qrBoundSmall = smallBound;

                    Log.d(TAG, "Java: showCameraDialog usable size=" + screenW + "x" + screenH + " bounds large=" + largeBound + " small=" + smallBound);

                    int cameraW = 640, cameraH = 480;

                    Log.d(TAG, "Java: camera preview resolution=" + cameraW + "x" + cameraH);

                    int[] dialog = computeDialogSize(screenW, screenH, cameraW, cameraH);
                    int dialogW = dialog[0];
                    int dialogH = dialog[1];

                    Log.d(TAG, "Java: showCameraDialog calculated dialog size=" + dialogW + "x" + dialogH);

                    final int finalW = dialogW;
                    final int finalH = dialogH;

                    final Dialog d = new Dialog(act);
                    d.requestWindowFeature(Window.FEATURE_NO_TITLE);
                    if (d.getWindow() != null) {
                        d.getWindow().setLayout(finalW, finalH);
                        d.getWindow().setBackgroundDrawable(
                                new android.graphics.drawable.ColorDrawable(Color.BLACK));
                        // Центрируем диалог
                        d.getWindow().setGravity(Gravity.CENTER);
                    }
                    
                    SurfaceView surface = new SurfaceView(act);
                    surface.setLayoutParams(new ViewGroup.LayoutParams(finalW, finalH));
                    // surface.setZOrderMediaOverlay(true);
                    final SurfaceView sv = surface;
                    sv.getHolder().addCallback(new SurfaceHolder.Callback() {
                        public void surfaceCreated(SurfaceHolder h) {
                            Log.d(TAG, "Java: surfaceCreated, setting fixed size " + finalW + "x" + finalH);
                            h.setFixedSize(finalW, finalH);
                            startCameraOnThread(h);
                        }
                        public void surfaceChanged(SurfaceHolder h, int f, int w, int hh) {
                            Log.d(TAG, "Java: surfaceChanged format=" + f + " size=" + w + "x" + hh);
                        }
                        public void surfaceDestroyed(SurfaceHolder h) {
                            Log.d(TAG, "Java: surfaceDestroyed");
                            dismissCameraDialog();
                        }
                    });

                    FrameLayout root = new FrameLayout(act);
                    root.setLayoutParams(new ViewGroup.LayoutParams(finalW, finalH));
                    root.setBackgroundColor(Color.TRANSPARENT);
                    
                    // 1. Добавляем превью камеры вниз
                    root.addView(sv);

                    // 2. Добавляем полупрозрачный оверлей с прозрачным окном поверх камеры
                    QrOverlayView overlayView = new QrOverlayView(act);
                    // overlayView.setLayoutParams(new ViewGroup.LayoutParams(finalW, finalH));
                    overlayView.setLayoutParams(new ViewGroup.LayoutParams(
                        ViewGroup.LayoutParams.MATCH_PARENT, 
                        ViewGroup.LayoutParams.MATCH_PARENT
                    ));
                     root.addView(overlayView);

                    overlayView.setOnClickListener(new View.OnClickListener() {
                        @Override
                        public void onClick(View v) { toggleCameraFlash(); }
                    });

                    qrSurface = sv;
                    d.setContentView(root);
                    d.setCancelable(true);
                    d.setOnCancelListener(new DialogInterface.OnCancelListener() {
                        public void onCancel(DialogInterface di) { 
                            Log.d(TAG, "Java: dialog cancelled");
                            cancelCameraDialog(); 
                        }
                    });
                    qrDialog = d;
                    qrDialogShown = true;
                    d.show();
                    Log.d(TAG, "Java: showCameraDialog shown successfully " + finalW + "x" + finalH);
                } catch (Throwable t) {
                    Log.e(TAG, "Java: showCameraDialog failed: " + t.getMessage(), t);
                    failCameraOpen();
                }
            }
        });
    }
    
    // Cancel (Cancel button / hardware Back): dismiss + release, then tell Go.
    private static void cancelCameraDialog() {
        dismissCameraDialog();
        if (goNativeActivity != null) goNativeActivity.lifecycleEvent("qrCancel");
    }

    private static void startCameraOnThread(final SurfaceHolder h) {
        ensureCameraThread();
        qrCameraHandler.post(new Runnable() {
            @Override
            public void run() { startCameraWithHolder(h); }
        });
    }

    // Open + configure the back camera against the given (real) SurfaceHolder
    // and start streaming. Runs on the camera HandlerThread.
    private static boolean startCameraWithHolder(SurfaceHolder h) {
        if (qrCamera != null) return true; // already running
        int width = 0, height = 0;
        int rotate = 90; // display orientation (refined below; hoisted for sizeDialogToCamera)
        try {
            int id = findBackCameraId();
            if (id < 0) {
                Log.e(TAG, "Java: startCamera: no camera available");
                failCameraOpen();
                return false;
            }
            Camera c = Camera.open(id);

            try {
                Camera.Parameters params = c.getParameters();
                try {
                    params.setPreviewFormat(ImageFormat.NV21);
                } catch (Throwable ignored) {}

                try {
                    List<Camera.Size> sizes = params.getSupportedPreviewSizes();
                    int lg = qrBoundLarge > 0 ? qrBoundLarge : 640;
                    int sm = qrBoundSmall > 0 ? qrBoundSmall : 480;
                    Camera.Size chosen = choosePreviewSize(sizes, lg, sm);
                    if (chosen != null) {
                        params.setPreviewSize(chosen.width, chosen.height);
                        width = chosen.width;
                        height = chosen.height;
                    }
                } catch (Throwable ignored) {}
                try {
                    List<String> modes = params.getSupportedFocusModes();
                    if (modes != null && modes.contains(Camera.Parameters.FOCUS_MODE_CONTINUOUS_PICTURE)) {
                        params.setFocusMode(Camera.Parameters.FOCUS_MODE_CONTINUOUS_PICTURE);
                    }
                } catch (Throwable ignored) {}
                try {
                    List<String> flashModes = params.getSupportedFlashModes();
                    qrFlashTorchSupported = flashModes != null
                            && flashModes.contains(Camera.Parameters.FLASH_MODE_TORCH);
                } catch (Throwable ignored) { qrFlashTorchSupported = false; }
                // Preview FPS range — pick the range with the highest max, and among
                // those the smallest min (most flexible), so auto-exposure can adapt.
                // Forcing the fixed 30000-30000 (the old tie-break) is fragile on old
                // HALs and is what produced the ~15 fps / freeze on Android 9/10.
                try {
                    List<int[]> ranges = params.getSupportedPreviewFpsRange();
                    int[] picked = null;
                    if (ranges != null) {
                        for (int[] r : ranges) {
                            if (r == null || r.length < 2) continue;
                            if (picked == null || r[1] > picked[1]
                                    || (r[1] == picked[1] && r[0] < picked[0])) {
                                picked = r;
                            }
                        }
                    }
                    if (picked != null) {
                        params.setPreviewFpsRange(picked[0], picked[1]);
                        Log.d(TAG, "Java: previewFpsRange " + picked[0] + "-" + picked[1]);
                    }
                } catch (Throwable ignored) {}
                try {
                    c.setParameters(params);
                } catch (Throwable ignored) {}
            } catch (Throwable t) {
                Log.e(TAG, "Java: startCamera configure failed: " + t.getMessage());
            }

            if (width == 0 || height == 0) {
                Camera.Parameters p = c.getParameters();
                if (p != null) {
                    Camera.Size ps = p.getPreviewSize();
                    if (ps != null) {
                        width = ps.width;
                        height = ps.height;
                    }
                }
            }
            try {
                Camera.CameraInfo info = new Camera.CameraInfo();
                Camera.getCameraInfo(id, info);
                qrSensorOrientJava = info.orientation;
                // Back-camera formula (Android docs): rotate by
                // (sensorOrientation - displayRotation). getDeviceRotation() is the
                // same source the pre-change Go qrRot used (=> 1x 90 deg CW in
                // portrait), so preview and decode agree. Portrait => 90 deg CW.
                int degrees = getDeviceRotation();
                if (degrees < 0) degrees = 0;
                rotate = (info.orientation - degrees + 360) % 360;
                c.setDisplayOrientation(rotate);
            } catch (Throwable ignored) {}

            // Real, consumed native surface: the SurfaceView's Surface (a separate
            // window) consumes the preview, so the capture pipeline never stalls
            // (unlike the old dummy unconsumed SurfaceTexture(0)).
            try {
                c.setPreviewDisplay(h);
                Log.d(TAG, "Java: startCamera setPreviewDisplay ok");
            } catch (Throwable t) {
                Log.e(TAG, "Java: startCamera setPreviewDisplay failed: " + t.getMessage());
            }

            qrPreviewWidth = width;
            qrPreviewHeight = height;
            
            updateDialogSizeToCameraResolution(width, height);
            
            int side = Math.max(1, Math.min(width, height));
            qrSquareSide = side;
            qrSquareBuf = new byte[side * side];
            qrFrameCount = 0;
            qrLastDecodeFeedMs = 0;
            c.setPreviewCallbackWithBuffer(qrPreviewCallback);
            int bufSize = Math.max(1, width) * Math.max(1, height)
                * ImageFormat.getBitsPerPixel(ImageFormat.NV21) / 8;
            // Prime several buffers so the camera always has one ready to fill
            // (one buffer can starve the capture pipeline). On `keep` the returned
            // buffer is re-added, keeping the pool topped up.
            for (int i = 0; i < 3; i++) c.addCallbackBuffer(new byte[bufSize]);

            qrCamera = c;
            qrCameraRunning = true;
            c.startPreview();
            // Уважать последний тап пользователя (напр. тап в окне до открытия камеры).
            applyCameraFlash(c);
            Log.d(TAG, "Java: startCamera " + width + "x" + height);
            return true;
        } catch (Throwable t) {
            Log.e(TAG, "Java: startCamera failed: " + t.getMessage());
            qrPreviewWidth = 0;
            qrPreviewHeight = 0;
            failCameraOpen();
            return false;
        }
    }

    private static void failCameraOpen() {
        qrCameraRunning = false;
        qrDialogShown = false;
        dismissCameraDialog();
        if (goNativeActivity != null) goNativeActivity.lifecycleEvent("cameraOpenFailed");
    }

    // Переключить фонарик. Вызывается из click-listener'а оверлея (UI-поток).
    // Инвертирует желаемое состояние и применяет на камерном потоке (все camera-операции
    // только на qrCameraHandler). Молчаливый no-op, если задняя камера без FLASH_MODE_TORCH.
    private static void toggleCameraFlash() {
        if (!qrFlashTorchSupported) return;
        qrFlashOn = !qrFlashOn;
        final boolean on = qrFlashOn;
        ensureCameraThread();
        qrCameraHandler.post(new Runnable() {
            @Override
            public void run() {
                Camera c = qrCamera;
                if (c != null) applyCameraFlash(c, on);
            }
        });
    }

    // Применить текущее qrFlashOn к открытой камере. Камерный поток.
    private static void applyCameraFlash(Camera c) { applyCameraFlash(c, qrFlashOn); }

    private static void applyCameraFlash(Camera c, boolean on) {
        try {
            Camera.Parameters params = c.getParameters();
            params.setFlashMode(on ? Camera.Parameters.FLASH_MODE_TORCH
                                   : Camera.Parameters.FLASH_MODE_OFF);
            c.setParameters(params);
        } catch (Throwable t) {
            Log.e(TAG, "Java: setFlashMode failed: " + t.getMessage());
        }
    }

    // Recompute + apply the preview display orientation for the current device
    // rotation. Called from onConfigurationChanged while the camera Dialog is up,
    // since configChanges="orientation|..." absorbs rotation without recreating
    // the activity (so a single setDisplayOrientation at open would go stale).
    // Mirrors the pre-change Go behavior of re-reading getDeviceRotation per
    // frame. setDisplayOrientation only rotates the surface display, never the
    // onPreviewFrame bytes (Go's qrRot keeps decode upright independently).
    private static void reapplyPreviewOrientation() {
        final Camera c = qrCamera;
        if (c == null) return;
        int degrees = getDeviceRotation();
        if (degrees < 0) degrees = 0;
        final int rotate = (qrSensorOrientJava - degrees + 360) % 360;
        qrCameraHandler.post(new Runnable() {
            @Override
            public void run() { try { c.setDisplayOrientation(rotate); } catch (Throwable ignored) {} }
        });
    }

    private static void updateDialogSizeToCameraResolution(final int cameraW, final int cameraH) {
        if (goNativeActivity == null) return;
        goNativeActivity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                if (qrDialog == null || !qrDialog.isShowing()) return;
                
                try {
                    int[] usable = getUsableScreenSize();
                    int screenW = usable[0];
                    int screenH = usable[1];

                    int[] dialog = computeDialogSize(screenW, screenH, cameraW, cameraH);
                    int dialogW = dialog[0];
                    int dialogH = dialog[1];

                    
                    
                    Window window = qrDialog.getWindow();
                    if (window != null) {
                        window.setLayout(dialogW, dialogH);
                        Log.d(TAG, "Java: dialog resized to camera resolution " + dialogW + "x" + dialogH);
                    }
                    
                    if (qrSurface != null) {
                        ViewGroup.LayoutParams params = qrSurface.getLayoutParams();
                        if (params.width != dialogW || params.height != dialogH) {
                            params.width = dialogW;
                            params.height = dialogH;
                            qrSurface.setLayoutParams(params);
                            
                            SurfaceHolder holder = qrSurface.getHolder();
                            if (holder != null) {
                                holder.setFixedSize(dialogW, dialogH);
                                Log.d(TAG, "Java: surface holder resized to " + dialogW + "x" + dialogH);
                            }
                        }
                    }
                    
                } catch (Throwable t) {
                    Log.e(TAG, "Java: updateDialogSizeToCameraResolution failed: " + t.getMessage());
                }
            }
        });
    }

    // Idempotent: dismiss the native camera Dialog and release the camera. Safe
    // to call when nothing is open. Called from Go (decode hit / pause), from
    // cancel, and from onPause.
    static void dismissCameraDialog() {
        qrDialogShown = false;
        ensureCameraThread();
        qrCameraHandler.post(new Runnable() {
            @Override
            public void run() { stopCamera(); }
        });
        if (goNativeActivity != null) {
            goNativeActivity.runOnUiThread(new Runnable() {
                @Override
                public void run() {
                    Dialog d = qrDialog;
                    qrDialog = null;
                    qrSurface = null;
                    if (d != null) {
                        try { if (d.isShowing()) d.dismiss(); } catch (Throwable ignored) {}
                    }
                    Log.d(TAG, "Java: dismissCameraDialog done");
                }
            });
        } else {
            qrDialog = null;
            qrSurface = null;
        }
    }

    static void stopCamera() {
        qrCameraRunning = false;
        Camera c = qrCamera;
        qrCamera = null;
        qrSquareBuf = null;
        qrSquareSide = 0;
        qrBoundLarge = 0;
        qrBoundSmall = 0;
        qrFlashOn = false;
        qrFlashTorchSupported = false;
        if (c == null) return;
        try { c.setPreviewCallbackWithBuffer(null); } catch (Throwable ignored) {}
        try { c.stopPreview(); } catch (Throwable ignored) {}
        try { c.release(); } catch (Throwable ignored) {}
        Log.d(TAG, "Java: stopCamera");
    }

    // Extract the centered square of the Y plane (the first w*h bytes of the NV21
    // buffer) into the reused qrSquareBuf and hand it to Go as a side x side
    // square. Falls back to passing the raw NV21 frame when the buffer is
    // unavailable or the frame is too small (Go's cropCenterSquare then squares
    // it). Returns true to keep streaming, false to stop.
    private boolean feedSquareFrame(byte[] data) {
        int pw = qrPreviewWidth;
        int ph = qrPreviewHeight;
        int side = qrSquareSide;
        byte[] buf = qrSquareBuf;
        if (buf != null && side > 0 && pw > 0 && ph > 0 && data.length >= pw * ph
                && pw >= side && ph >= side) {
            int xoff = (pw - side) / 2;
            int yoff = (ph - side) / 2;
            for (int r = 0; r < side; r++) {
                System.arraycopy(data, (yoff + r) * pw + xoff, buf, r * side, side);
            }
            return cameraFrame(buf, side, side);
        }
        return cameraFrame(data, pw, ph);
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

    private static int appThemeMode = 0;
    private static boolean systemDark = false;

    static void setAppThemeMode(int mode) {
        appThemeMode = mode;
        applySystemBarsAppearance();
    }

    static void applySystemBarsAppearance() {
        final GoNativeActivity act = goNativeActivity;
        if (act == null) return;
        final boolean lightBg = (appThemeMode == 1) || (appThemeMode == 0 && !systemDark);
        act.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                Window window = act.getWindow();
                if (window == null) return;
                try { window.setStatusBarColor(lightBg ? Color.WHITE : Color.BLACK); } catch (Throwable ignored) {}
                try { window.setNavigationBarColor(lightBg ? Color.WHITE : Color.BLACK); } catch (Throwable ignored) {}
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                    WindowInsetsController c = window.getInsetsController();
                    if (c != null) {
                        int mask = WindowInsetsController.APPEARANCE_LIGHT_STATUS_BARS
                                 | WindowInsetsController.APPEARANCE_LIGHT_NAVIGATION_BARS;
                        c.setSystemBarsAppearance(lightBg ? mask : 0, mask);
                    }
                } else {
                    View v = window.getDecorView();
                    int f = v.getSystemUiVisibility();
                    if (lightBg) f |= View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;
                    else f &= ~View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                        if (lightBg) f |= View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR;
                        else f &= ~View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR;
                    }
                    v.setSystemUiVisibility(f);
                }
            }
        });
    }

    static String getFileName(String uriStr) {
        try {
            android.net.Uri uri = android.net.Uri.parse(uriStr);
            String[] projection = {android.provider.OpenableColumns.DISPLAY_NAME};
            android.database.Cursor cursor = goNativeActivity.getContentResolver().query(uri, projection, null, null, null);
            if (cursor != null) {
                try {
                    if (cursor.moveToFirst()) return cursor.getString(0);
                } finally { cursor.close(); }
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

    static boolean openIntent(String intentStr) {
        try {
            android.content.Intent intent = android.content.Intent.parseUri(intentStr, android.content.Intent.URI_INTENT_SCHEME);
            goNativeActivity.startActivityForResult(intent, INTENT_OPEN_CODE);
            return true;
        } catch (Exception e) {
            Log.e(TAG, "Java: openIntent failed: " + e.getMessage());
            return false;
        }
    }

    // resolveIntent парсит intent-URI и возвращает описание того, кто обработает его
    // (default-компонент + список кандидатов), не запуская activity.
    static String resolveIntent(String intentStr) {
        try {
            android.content.Intent intent = android.content.Intent.parseUri(intentStr, android.content.Intent.URI_INTENT_SCHEME);
            android.content.pm.PackageManager pm = goNativeActivity.getPackageManager();
            android.content.pm.ResolveInfo def =
                pm.resolveActivity(intent, android.content.pm.PackageManager.MATCH_DEFAULT_ONLY);
            java.util.List<android.content.pm.ResolveInfo> all = pm.queryIntentActivities(intent, 0);
            StringBuilder sb = new StringBuilder();
            sb.append("default=").append(def == null ? "none/chooser"
                : (def.activityInfo.applicationInfo.packageName + "/" + def.activityInfo.name));
            sb.append("; candidates=").append(all == null ? 0 : all.size()).append(" [");
            if (all != null) {
                for (int i = 0; i < all.size(); i++) {
                    if (i > 0) sb.append(", ");
                    sb.append(all.get(i).activityInfo.applicationInfo.packageName);
                }
            }
            sb.append("]");
            Log.d(TAG, "Java: resolveIntent " + intentStr + " => " + sb.toString());
            return sb.toString();
        } catch (Exception e) {
            Log.e(TAG, "Java: resolveIntent failed: " + e.getMessage());
            return "error: " + e.getMessage();
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

    static String createFileInDownloads(String fileName, String mimeType) {
        try {
            if (mimeType == null || mimeType.isEmpty()) {
                mimeType = "application/octet-stream";
            }
            if (Build.VERSION.SDK_INT >= 29) {
                return createFileInDownloadsModern(fileName, mimeType);
            } else {
                if (goNativeActivity.checkSelfPermission("android.permission.WRITE_EXTERNAL_STORAGE") != 0) {
                    goNativeActivity.requestPermissions(new String[]{
                        "android.permission.READ_EXTERNAL_STORAGE",
                        "android.permission.WRITE_EXTERNAL_STORAGE"
                    }, 123);
                    return null;
                }
                return createFileInDownloadsLegacy(fileName);
            }
        } catch (Exception e) {
            Log.e(TAG, "Java: createFileInDownloads failed: " + e.getMessage());
            return null;
        }
    }

    static String createFileInDownloadsModern(String fileName, String mimeType) {
        try {
            android.content.ContentResolver resolver = goNativeActivity.getContentResolver();
            android.content.ContentValues values = new android.content.ContentValues();
            String dirPath = null;
            String baseName = fileName;
            int lastSlash = fileName.lastIndexOf('/');
            if (lastSlash >= 0) {
                dirPath = fileName.substring(0, lastSlash);
                baseName = fileName.substring(lastSlash + 1);
                createDirectoriesInMediaStore(resolver, dirPath);
            }
            values.put("_display_name", baseName);
            values.put("mime_type", mimeType);
            if (dirPath != null && !dirPath.isEmpty()) {
                values.put("relative_path", "Download/" + dirPath);
            } else {
                values.put("relative_path", "Download");
            }
            android.net.Uri uri = resolver.insert(android.provider.MediaStore.Downloads.EXTERNAL_CONTENT_URI, values);
            return uri != null ? uri.toString() : null;
        } catch (Exception e) {
            Log.e(TAG, "Java: createFileInDownloadsModern failed: " + e.getMessage());
            return null;
        }
    }

    static void createDirectoriesInMediaStore(android.content.ContentResolver resolver, String relativePath) {
        if (relativePath == null || relativePath.isEmpty()) return;
        try {
            String[] parts = relativePath.split("/");
            StringBuilder currentPath = new StringBuilder();
            for (String part : parts) {
                if (currentPath.length() > 0) currentPath.append("/");
                currentPath.append(part);
                android.content.ContentValues values = new android.content.ContentValues();
                values.put("_display_name", currentPath.toString());
                values.put("mime_type", "vnd.android.document/directory");
                values.put("relative_path", "Download");
                try {
                    resolver.insert(android.provider.MediaStore.Downloads.EXTERNAL_CONTENT_URI, values);
                } catch (Exception ignored) {}
            }
        } catch (Exception e) {
            Log.e(TAG, "Java: createDirectoriesInMediaStore failed: " + e.getMessage());
        }
    }

    static String createFileInDownloadsLegacy(String fileName) {
        try {
            java.io.File downloadsDir = android.os.Environment.getExternalStoragePublicDirectory(android.os.Environment.DIRECTORY_DOWNLOADS);
            java.io.File file = new java.io.File(downloadsDir, fileName);
            file.createNewFile();
            return file.toURI().toString();
        } catch (Exception e) {
            Log.e(TAG, "Java: createFileInDownloadsLegacy failed: " + e.getMessage());
            return null;
        }
    }

    static long getSize(String uriStr) {
        try {
            android.net.Uri uri = android.net.Uri.parse(uriStr);
            android.content.ContentResolver resolver = goNativeActivity.getContentResolver();
            String[] projection = {android.provider.OpenableColumns.SIZE};
            android.database.Cursor cursor = resolver.query(uri, projection, null, null, null);
            if (cursor != null) {
                try {
                    if (cursor.moveToFirst()) {
                        return cursor.getLong(0);
                    }
                } finally {
                    cursor.close();
                }
            }
        } catch (Exception e) {
            Log.e(TAG, "Java: getSize failed: " + e.getMessage());
        }
        return -1;
    }

    static long getModTime(String uriStr) {
        try {
            android.net.Uri uri = android.net.Uri.parse(uriStr);
            android.content.ContentResolver resolver = goNativeActivity.getContentResolver();
            try {
                String[] projection = {"last_modified"};
                android.database.Cursor cursor = resolver.query(uri, projection, null, null, null);
                if (cursor != null) {
                    try {
                        if (cursor.moveToFirst()) return cursor.getLong(0);
                    } finally { cursor.close(); }
                }
            } catch (Exception ignored) {}
            try {
                android.database.Cursor cursor = resolver.query(uri, new String[]{"date_modified"}, null, null, null);
                if (cursor != null) {
                    try {
                        if (cursor.moveToFirst()) return cursor.getLong(0);
                    } finally { cursor.close(); }
                }
            } catch (Exception ignored) {}
        } catch (Exception e) {
            Log.e(TAG, "Java: getModTime failed: " + e.getMessage());
        }
        return -1;
    }

    static int countChildren(String uriStr) {
        try {
            android.net.Uri uri = android.net.Uri.parse(uriStr);
            android.content.ContentResolver resolver = goNativeActivity.getContentResolver();
            android.net.Uri childUri = null;
            try {
                String treeDocId = android.provider.DocumentsContract.getTreeDocumentId(uri);
                childUri = android.provider.DocumentsContract.buildChildDocumentsUriUsingTree(uri, treeDocId);
            } catch (Exception e1) {
                try {
                    String docId = android.provider.DocumentsContract.getDocumentId(uri);
                    childUri = android.provider.DocumentsContract.buildChildDocumentsUri(uri.getAuthority(), docId);
                } catch (Exception e2) {
                    childUri = uri;
                }
            }
            android.database.Cursor cursor = resolver.query(childUri, null, null, null, null);
            if (cursor != null) {
                try {
                    return cursor.getCount();
                } finally { cursor.close(); }
            }
        } catch (Exception e) {
            Log.e(TAG, "Java: countChildren failed: " + e.getMessage());
        }
        return -1;
    }

    static String getChildrenURIs(String uriStr) {
        try {
            android.net.Uri uri = android.net.Uri.parse(uriStr);
            android.content.ContentResolver resolver = goNativeActivity.getContentResolver();
            android.net.Uri childUri = null;
            try {
                String treeDocId = android.provider.DocumentsContract.getTreeDocumentId(uri);
                childUri = android.provider.DocumentsContract.buildChildDocumentsUriUsingTree(uri, treeDocId);
                android.database.Cursor testCursor = resolver.query(childUri, new String[]{"_display_name", "document_id"}, null, null, null);
                if (testCursor != null) { testCursor.close(); }
                else {
                    testCursor = resolver.query(childUri, null, null, null, null);
                    if (testCursor != null) { testCursor.close(); }
                    else { childUri = null; }
                }
            } catch (Exception e1) {
                childUri = null;
                try {
                    String docId = android.provider.DocumentsContract.getDocumentId(uri);
                    childUri = android.provider.DocumentsContract.buildChildDocumentsUri(uri.getAuthority(), docId);
                    android.database.Cursor testCursor = resolver.query(childUri, new String[]{"document_id"}, null, null, null);
                    if (testCursor != null) { testCursor.close(); }
                    else { childUri = null; }
                } catch (Exception e2) {
                    childUri = null;
                    return "";
                }
            }
            if (childUri == null) return "";
            android.database.Cursor cursor = resolver.query(childUri, new String[]{"document_id"}, null, null, null);
            if (cursor == null) return "";
            return processChildrenCursor(cursor, uri);
        } catch (Exception e) {
            Log.e(TAG, "Java: getChildrenURIs failed: " + e.getMessage());
        }
        return "";
    }

    private static String processChildrenCursor(android.database.Cursor cursor, android.net.Uri treeUri) {
        StringBuilder sb = new StringBuilder();
        try {
            int docIdCol = cursor.getColumnIndex("document_id");
            if (docIdCol < 0) docIdCol = 0;
            boolean first = true;
            while (cursor.moveToNext()) {
                String docId = cursor.getString(docIdCol);
                if (docId != null) {
                    android.net.Uri childDocUri = null;
                    try {
                        childDocUri = android.provider.DocumentsContract.buildDocumentUriUsingTree(treeUri, docId);
                    } catch (Exception ignored) {}
                    if (childDocUri == null) {
                        try {
                            childDocUri = android.provider.DocumentsContract.buildChildDocumentsUriUsingTree(treeUri, docId);
                        } catch (Exception ignored) {}
                    }
                    if (childDocUri == null) {
                        try {
                            childDocUri = treeUri.buildUpon().appendPath(docId).build();
                        } catch (Exception ignored) {}
                    }
                    if (childDocUri != null) {
                        if (!first) sb.append("|");
                        sb.append(childDocUri.toString());
                        first = false;
                    }
                }
            }
        } finally { cursor.close(); }
        return sb.toString();
    }

    static String createFileInTree(String treeUri, String fileName, String mimeType) {
        try {
            if (mimeType == null || mimeType.isEmpty()) mimeType = "application/octet-stream";
            android.net.Uri uri = android.net.Uri.parse(treeUri);
            android.content.ContentResolver resolver = goNativeActivity.getContentResolver();
            String treeDocId = android.provider.DocumentsContract.getTreeDocumentId(uri);
            android.net.Uri childDocsUri = android.provider.DocumentsContract.buildChildDocumentsUriUsingTree(uri, treeDocId);
            android.net.Uri newFileUri = android.provider.DocumentsContract.createDocument(resolver, childDocsUri, mimeType, fileName);
            if (newFileUri != null) return newFileUri.toString();
            return "error: createDocument returned null";
        } catch (Exception e) {
            String msg = e.getMessage();
            return msg != null ? "error: " + msg : "error: createFileInTree failed";
        }
    }

    static String resolveMediaStoreUri(String safUri) {
        if (Build.VERSION.SDK_INT < 29) return null;
        try {
            if (safUri == null || safUri.isEmpty()) return null;
            if (safUri.contains("content://media/")) return safUri;

            String decoded = java.net.URLDecoder.decode(safUri, "UTF-8");
            String path = null;

            if (decoded.contains("primary:") || decoded.contains("primary%3A")) {
                int idx = decoded.indexOf("primary:");
                if (idx == -1) idx = decoded.indexOf("primary%3A");
                if (idx == -1) return null;
                int start = decoded.indexOf(":", idx) + 1;
                if (start == 0) return null;
                path = decoded.substring(start);
            } else if (decoded.contains("raw:") || decoded.contains("raw%3A")) {
                int idx = decoded.indexOf("raw:");
                if (idx == -1) idx = decoded.indexOf("raw%3A");
                if (idx == -1) return null;
                int start = decoded.indexOf(":", idx) + 1;
                if (start == 0) return null;
                path = decoded.substring(start);
                path = java.net.URLDecoder.decode(path, "UTF-8");
            } else {
                return null;
            }

            if (path.startsWith("/storage/emulated/0/")) {
                path = path.substring("/storage/emulated/0/".length());
            }

            if (path.isEmpty()) return null;

            int lastSlash = path.lastIndexOf('/');
            String fileName = lastSlash >= 0 ? path.substring(lastSlash + 1) : path;

            android.content.ContentResolver resolver = goNativeActivity.getContentResolver();
            String collectionType;
            android.net.Uri collectionUri;

            if (path.startsWith("Download")) {
                collectionType = "downloads";
                collectionUri = android.provider.MediaStore.Downloads.EXTERNAL_CONTENT_URI;
            } else if (path.startsWith("Pictures") || path.startsWith("DCIM")) {
                collectionType = "images";
                collectionUri = android.provider.MediaStore.Images.Media.EXTERNAL_CONTENT_URI;
            } else if (path.startsWith("Movies")) {
                collectionType = "video";
                collectionUri = android.provider.MediaStore.Video.Media.EXTERNAL_CONTENT_URI;
            } else if (path.startsWith("Music") || path.startsWith("Alarms") || path.startsWith("Podcasts") || path.startsWith("Ringtones")) {
                collectionType = "audio";
                collectionUri = android.provider.MediaStore.Audio.Media.EXTERNAL_CONTENT_URI;
            } else {
                return null;
            }

            String[] projection = {"_id"};
            String selection = "_display_name = ?";
            String[] selectionArgs = {fileName};
            android.database.Cursor cursor = resolver.query(collectionUri, projection, selection, selectionArgs, null);
            if (cursor != null) {
                try {
                    while (cursor.moveToNext()) {
                        long id = cursor.getLong(0);
                        String resultUri = "content://media/external/" + collectionType + "/" + id;
                        Log.d(TAG, "resolveMediaStoreUri: " + safUri + " -> " + resultUri);
                        return resultUri;
                    }
                } finally {
                    cursor.close();
                }
            }
            Log.d(TAG, "resolveMediaStoreUri: not found for " + safUri + " path=" + path);
        } catch (Exception e) {
            Log.e(TAG, "resolveMediaStoreUri failed: " + e.getMessage());
        }
        return null;
    }

    static String createFileViaMediaStore(String collectionType, String relativePath, String fileName, String mimeType) {
        if (Build.VERSION.SDK_INT < 29) return null;
        try {
            if (mimeType == null || mimeType.isEmpty()) mimeType = "application/octet-stream";
            android.content.ContentResolver resolver = goNativeActivity.getContentResolver();
            android.content.ContentValues values = new android.content.ContentValues();
            values.put("_display_name", fileName);
            values.put("mime_type", mimeType);
            if (relativePath != null && !relativePath.isEmpty()) {
                values.put("relative_path", relativePath);
            }

            android.net.Uri collectionUri;
            if ("images".equals(collectionType)) {
                collectionUri = android.provider.MediaStore.Images.Media.EXTERNAL_CONTENT_URI;
            } else if ("video".equals(collectionType)) {
                collectionUri = android.provider.MediaStore.Video.Media.EXTERNAL_CONTENT_URI;
            } else if ("audio".equals(collectionType)) {
                collectionUri = android.provider.MediaStore.Audio.Media.EXTERNAL_CONTENT_URI;
            } else {
                collectionUri = android.provider.MediaStore.Downloads.EXTERNAL_CONTENT_URI;
            }

            android.net.Uri uri = resolver.insert(collectionUri, values);
            if (uri != null) {
                Log.d(TAG, "createFileViaMediaStore: " + uri.toString());
                return uri.toString();
            }
            return null;
        } catch (Exception e) {
            Log.e(TAG, "createFileViaMediaStore failed: " + e.getMessage());
            return null;
        }
    }

    static boolean isIntentSupported(String action, String mimeType) {
        try {
            android.content.Intent intent = new android.content.Intent(action);
            intent.addCategory(android.content.Intent.CATEGORY_DEFAULT);
            if (!action.equals(android.content.Intent.ACTION_OPEN_DOCUMENT_TREE)) {
                intent.addCategory(android.content.Intent.CATEGORY_OPENABLE);
            }
            if (mimeType != null && !mimeType.isEmpty()) {
                intent.setType(mimeType);
            }
            java.util.List<android.content.pm.ResolveInfo> activities = goNativeActivity.getPackageManager().queryIntentActivities(intent, android.content.pm.PackageManager.MATCH_DEFAULT_ONLY);
            return activities != null && !activities.isEmpty();
        } catch (Exception e) {
            Log.e(TAG, "Java: isIntentSupported failed: " + e.getMessage());
        }
        return false;
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
        if (requestCode == INTENT_OPEN_CODE) {
            return;
        }
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
        
        // Обновляем размер диалога при повороте
        if (qrDialog != null && qrDialog.isShowing()) {
            updateDialogSizeToCameraResolution(qrPreviewWidth, qrPreviewHeight);

        }
        
        if (qrDialogShown) reapplyPreviewOrientation();
    }


    protected void updateTheme(Configuration config) {
        boolean dark = (config.uiMode & Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES;
        setDarkMode(dark);
        systemDark = dark;
        applySystemBarsAppearance();
    }

    // Re-applies the system dark state on foreground return. On API 29 (Android 10) a uiMode change
    // toggled from the quick-settings shade is not delivered as onConfigurationChanged before/at
    // resume, leaving the Fyne content theme and the status bar stale. No-op when unchanged.
    private void reconcileSystemTheme() {
        Configuration cfg = getResources().getConfiguration();
        boolean dark = (cfg.uiMode & Configuration.UI_MODE_NIGHT_MASK) == Configuration.UI_MODE_NIGHT_YES;
        if (dark != systemDark) {
            updateTheme(cfg);
        }
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
        lifecycleEvent("UserLeaveHint");
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
        reconcileSystemTheme();
    }

    @Override
    protected void onPause() {
        Log.d(TAG, "Java: onPause");
        lifecycleEvent("pause");
        try { dismissCameraDialog(); } catch (Throwable ignored) {} // release + dismiss even if Go is slow
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
        if (requestCode == CAMERA_PERMISSION_CODE) {
            boolean granted = grantResults != null && grantResults.length > 0
                && grantResults[0] == PackageManager.PERMISSION_GRANTED;
            Log.d(TAG, "Java: camera permission granted=" + granted);
            lifecycleEvent(granted ? "cameraPermissionGranted" : "cameraPermissionDenied");
            return;
        }
        Log.d(TAG, "Java: onRequestPermissionsResult requestCode=" + requestCode + ", grantResults=" + grantResults[0]);
        if (requestCode == 123 && pendingIntentURIs != null) {
            boolean granted = false;
            for (int result : grantResults) {
                if (result == PackageManager.PERMISSION_GRANTED) granted = true;
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