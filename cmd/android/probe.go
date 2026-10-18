package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The generated instrumentation exercises real keyboard/touch scene advances
// and the Activity lifecycle. It contains no presentation logic.
const touchProbe = `package com.olivierh.dma40;
import android.app.Instrumentation;
import android.app.Activity;
import android.content.Intent;
import android.graphics.Bitmap;
import android.os.Bundle;
import android.os.SystemClock;
import android.view.KeyEvent;
import android.view.MotionEvent;
import org.json.JSONObject;
import com.olivierh.dma40.mobile.Mobile;
import java.io.File;
import java.io.FileOutputStream;
public final class TouchProbe extends Instrumentation {
 private JSONObject state() throws Exception { return new JSONObject(Mobile.state()); }
 private void require(boolean ok, String why) { if (!ok) throw new AssertionError(why); }
 private void waitTicks(int count) throws Exception {
  long tick=state().getLong("tick"),end=SystemClock.uptimeMillis()+4000;
  while(state().getLong("tick")<tick+count && SystemClock.uptimeMillis()<end) SystemClock.sleep(10);
  require(state().getLong("tick")>=tick+count,"Go timeline stopped advancing");
 }
 private void key(int code,long hold) {
  sendKeySync(new KeyEvent(KeyEvent.ACTION_DOWN,code));SystemClock.sleep(hold);
  sendKeySync(new KeyEvent(KeyEvent.ACTION_UP,code));SystemClock.sleep(120);
 }
 private void waitStage(String stage) throws Exception {
  long end=SystemClock.uptimeMillis()+5000;
  while(!state().getString("stage").equals(stage) && SystemClock.uptimeMillis()<end) SystemClock.sleep(20);
  require(state().getString("stage").equals(stage),"Space did not reach "+stage);
  require(!state().getBoolean("paused"),"Space paused instead of advancing");
 }
 private void advance() throws Exception {
  JSONObject before=state();key(KeyEvent.KEYCODE_SPACE,80);
  long end=SystemClock.uptimeMillis()+5000;
  while(state().getString("stage").equals(before.getString("stage")) && state().getInt("local_tick")<before.getInt("local_tick")+25 && SystemClock.uptimeMillis()<end) SystemClock.sleep(20);
 }
 private void tap(float x,float y) {
  long now=SystemClock.uptimeMillis();
  sendPointerSync(MotionEvent.obtain(now,now,MotionEvent.ACTION_DOWN,x,y,0));SystemClock.sleep(60);
  sendPointerSync(MotionEvent.obtain(now,SystemClock.uptimeMillis(),MotionEvent.ACTION_UP,x,y,0));SystemClock.sleep(80);
 }
 @Override public void onCreate(Bundle args) {super.onCreate(args);start();}
 @Override public void onStart() {
  Bundle result=new Bundle();
  try {
   Intent intent=new Intent(getTargetContext(),MainActivity.class);
   intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK|Intent.FLAG_ACTIVITY_CLEAR_TASK);
   final Activity activity=startActivitySync(intent);
   waitForIdleSync();
   require(state().getString("stage").equals("opening"),"Check APK did not start with the intro");
   key(KeyEvent.KEYCODE_S,80);
   // A held key must advance once, leaving the next card visible.
   key(KeyEvent.KEYCODE_SPACE,350);waitStage("four-loader");
   advance();waitStage("four");
   advance();waitStage("four-fade");
   advance();waitStage("spoon-loader");
   advance();waitStage("spoon");
   for(int count=0;count<10 && !state().getString("effect").equals("voxel-flight");count++) advance();
   require(state().getString("effect").equals("voxel-flight"),"Scene advances skipped the volumetric flight");
   waitTicks(10);advance();
   require(state().getString("effect").equals("matrix"),"Space did not advance from the volume to Matrix");
   final android.graphics.Rect rect=new android.graphics.Rect();
   runOnMainSync(new Runnable(){public void run(){activity.getWindow().getDecorView().getGlobalVisibleRect(rect);}});
   tap(rect.centerX(),rect.centerY());
   require(state().getString("effect").equals("matrix"),"A single touch advanced the scene");
   tap(rect.centerX(),rect.centerY());SystemClock.sleep(250);
   require(state().getString("effect").equals("no-more-secrets"),"Double tap did not advance to the next scene");
   for(int count=0;count<12 && !state().getString("stage").equals("jelly");count++) advance();
   require(state().getString("stage").equals("jelly"),"Scene advances did not retain the Noname-to-Jelly handoff");
   waitTicks(5);
   runOnMainSync(new Runnable(){public void run(){callActivityOnPause(activity);}});
   SystemClock.sleep(150);long tick=state().getLong("tick");SystemClock.sleep(300);
   require(state().getLong("tick")==tick,"Background activity advanced the demo timeline");
   runOnMainSync(new Runnable(){public void run(){callActivityOnResume(activity);}});
   waitTicks(5);
   long deadline=SystemClock.uptimeMillis()+20000;
   while(state().getInt("local_tick")<14*50 && SystemClock.uptimeMillis()<deadline) SystemClock.sleep(20);
   require(state().getString("stage").equals("jelly") && state().getInt("local_tick")>=14*50,"Jelly did not reach its animated bounce");
   Bitmap image=getUiAutomation().takeScreenshot();require(image!=null,"Android screenshot is unavailable");
   FileOutputStream file=new FileOutputStream(new File(getTargetContext().getFilesDir(),"android-probe.png"));
   require(image.compress(Bitmap.CompressFormat.PNG,100,file),"Screenshot encoding failed");file.close();image.recycle();
   for(int count=0;count<6 && !state().getString("effect").equals("boids");count++) advance();
   require(state().getString("effect").equals("boids"),"Space did not reach the flock after the orbit");
   waitTicks(10);
   advance();require(state().getString("effect").equals("portrait-formation"),"Space skipped portrait assembly");
   advance();require(state().getString("effect").equals("portrait-bounce"),"Space skipped the portrait rebound");
   waitTicks(10);
   result.putString("success","true");result.putString("state",state().toString());finish(Activity.RESULT_OK,result);
  } catch(Throwable error) {result.putString("success","false");result.putString("error",error.toString());finish(Activity.RESULT_CANCELED,result);}
 }
}
`

func checkDevice(adb, serial, apk string) error {
	output := func(args ...string) ([]byte, error) {
		return exec.Command(adb, append([]string{"-s", serial}, args...)...).CombinedOutput()
	}
	raw, err := output("install", "-r", apk)
	fmt.Print(string(raw))
	if err != nil {
		return err
	}
	raw, err = output("shell", "am", "instrument", "-w", applicationID+"/.TouchProbe")
	if e := os.MkdirAll("captures", 0755); e != nil {
		return e
	}
	if e := os.WriteFile(filepath.Join("captures", "android-probe.txt"), raw, 0644); e != nil {
		return e
	}
	fmt.Print(string(raw))
	if err != nil {
		return err
	}
	if !strings.Contains(string(raw), "INSTRUMENTATION_RESULT: success=true") {
		return fmt.Errorf("Android scene-advance and lifecycle check did not report success")
	}
	raw, err = exec.Command(adb, "-s", serial, "exec-out", "run-as", applicationID, "cat", "files/android-probe.png").Output()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join("captures", "android-probe.png"), raw, 0644)
}
