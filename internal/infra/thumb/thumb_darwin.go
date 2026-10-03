// Package thumb makes small JPEGs: ImageIO on macOS (HEIC, RAW, EXIF rotation, subsampled decode), Go decoders elsewhere.
package thumb

/*
#cgo LDFLAGS: -framework ImageIO -framework CoreGraphics -framework CoreFoundation
#include <ImageIO/ImageIO.h>

static CFDataRef thumbnail(const void *data, long n, int px) {
	CFDataRef in = CFDataCreateWithBytesNoCopy(NULL, data, n, kCFAllocatorNull);
	CGImageSourceRef src = CGImageSourceCreateWithData(in, NULL);
	CFRelease(in);
	if (!src) return NULL;
	CFNumberRef size = CFNumberCreate(NULL, kCFNumberIntType, &px);
	const void *keys[] = {kCGImageSourceCreateThumbnailFromImageAlways, kCGImageSourceCreateThumbnailWithTransform, kCGImageSourceThumbnailMaxPixelSize};
	const void *vals[] = {kCFBooleanTrue, kCFBooleanTrue, size};
	CFDictionaryRef opts = CFDictionaryCreate(NULL, keys, vals, 3, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CGImageRef img = CGImageSourceCreateThumbnailAtIndex(src, 0, opts);
	CFRelease(opts);
	CFRelease(size);
	CFRelease(src);
	if (!img) return NULL;
	CFMutableDataRef out = CFDataCreateMutable(NULL, 0);
	CGImageDestinationRef dst = CGImageDestinationCreateWithData(out, CFSTR("public.jpeg"), 1, NULL);
	CGImageDestinationAddImage(dst, img, NULL);
	bool ok = CGImageDestinationFinalize(dst);
	CFRelease(dst);
	CGImageRelease(img);
	if (!ok) {
		CFRelease(out);
		return NULL;
	}
	return out;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

var errDecode = errors.New("not an image ImageIO can read")

// JPEG returns data as a JPEG at most px pixels on its longer side.
func JPEG(data []byte, px int) ([]byte, error) {
	if len(data) == 0 {
		return nil, errDecode
	}
	out := C.thumbnail(unsafe.Pointer(&data[0]), C.long(len(data)), C.int(px))
	if out == 0 {
		return nil, errDecode
	}
	defer C.CFRelease(C.CFTypeRef(out))
	return C.GoBytes(unsafe.Pointer(C.CFDataGetBytePtr(out)), C.int(C.CFDataGetLength(out))), nil
}
