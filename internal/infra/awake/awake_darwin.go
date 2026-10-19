// Package awake keeps the machine from idling to sleep, and macOS from napping the app, while transfers run.
package awake

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=11.0
#cgo LDFLAGS: -framework Foundation
#import <Foundation/Foundation.h>

static id<NSObject> activity;

static void hold(bool on) {
	if (on && !activity) {
		activity = [[NSProcessInfo processInfo] beginActivityWithOptions:NSActivityUserInitiated reason:@"Transferring files"];
	} else if (!on && activity) {
		[[NSProcessInfo processInfo] endActivity:activity];
		activity = nil;
	}
}
*/
import "C"

import "sync"

var mu sync.Mutex

// Hold begins (true) or ends (false) the activity; repeating the current state does nothing.
func Hold(on bool) {
	mu.Lock()
	defer mu.Unlock()
	C.hold(C.bool(on))
}
