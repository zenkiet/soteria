#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>

extern int goDragWrite(char *remote, char *dest, long long size, int dir);
extern void goDragEnded(void);

@interface DragSource : NSObject <NSDraggingSource, NSFilePromiseProviderDelegate>
@property(strong) NSOperationQueue *queue;
@end

@implementation DragSource
- (NSDragOperation)draggingSession:(NSDraggingSession *)session
    sourceOperationMaskForDraggingContext:(NSDraggingContext)context {
	return context == NSDraggingContextWithinApplication ? NSDragOperationNone : NSDragOperationCopy;
}

- (void)draggingSession:(NSDraggingSession *)session
           endedAtPoint:(NSPoint)screenPoint
              operation:(NSDragOperation)operation {
	goDragEnded();
}

- (NSString *)filePromiseProvider:(NSFilePromiseProvider *)provider fileNameForType:(NSString *)fileType {
	return provider.userInfo[@"name"];
}

- (void)filePromiseProvider:(NSFilePromiseProvider *)provider
            writePromiseToURL:(NSURL *)url
            completionHandler:(void (^)(NSError *))completionHandler {
	NSDictionary *f = provider.userInfo;
	int rc = goDragWrite((char *)[f[@"path"] UTF8String], (char *)[url.path UTF8String], [f[@"size"] longLongValue], [f[@"dir"] boolValue]);
	completionHandler(rc == 0 ? nil : [NSError errorWithDomain:@"soteria" code:rc userInfo:nil]);
}

- (NSOperationQueue *)operationQueueForFilePromiseProvider:(NSFilePromiseProvider *)provider {
	return self.queue;
}
@end

static DragSource *source;

int dragOut(void *window, const char *json) {
	NSEvent *event = [NSApp currentEvent];
	if (event.type != NSEventTypeLeftMouseDragged && event.type != NSEventTypeLeftMouseDown) {
		return 1;
	}
	NSView *view = nil;
	for (NSView *v in ((__bridge NSWindow *)window).contentView.subviews) {
		if ([v isKindOfClass:[WKWebView class]]) {
			view = v;
			break;
		}
	}
	if (!view) {
		return 2;
	}
	NSArray *files = [NSJSONSerialization JSONObjectWithData:[NSData dataWithBytes:json length:strlen(json)]
	                                                 options:0
	                                                   error:nil];
	if (!source) {
		source = [DragSource new];
		source.queue = [NSOperationQueue new];
	}
	NSPoint at = [view convertPoint:event.locationInWindow fromView:nil];
	NSMutableArray *items = [NSMutableArray new];
	for (NSDictionary *f in files) {
		UTType *type = [f[@"dir"] boolValue] ? UTTypeFolder
		                                     : ([UTType typeWithFilenameExtension:[f[@"name"] pathExtension]] ?: UTTypeData);
		NSFilePromiseProvider *p = [[NSFilePromiseProvider alloc] initWithFileType:type.identifier delegate:source];
		p.userInfo = f;
		NSDraggingItem *item = [[NSDraggingItem alloc] initWithPasteboardWriter:p];
		[item setDraggingFrame:NSMakeRect(at.x - 24, at.y - 24, 48, 48)
		              contents:[[NSWorkspace sharedWorkspace] iconForContentType:type]];
		[items addObject:item];
	}
	[view beginDraggingSessionWithItems:items event:event source:source];
	return 0;
}
