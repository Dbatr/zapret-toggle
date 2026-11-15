//go:build linux && cgo && (x11 || !wayland)

// Не собирается только с тегом wayland без x11 (чистый Wayland): как и GLFW, такая сборка
// не линкуется с libX11. Без тегов и с x11 всё как раньше.

package startupnotify

/*
#cgo LDFLAGS: -lX11
#include <stdlib.h>
#include <string.h>
#include <X11/Xlib.h>

// Как libstartup-notification: сообщение режется на куски по 20 байт; первый уходит
// с типом _NET_STARTUP_INFO_BEGIN, остальные — _NET_STARTUP_INFO; адресат — корневое окно
// с маской PropertyChangeMask, в поле window — своё временное окно.
static int zt_broadcast(const char *msg, int len) {
	Display *d = XOpenDisplay(NULL);
	if (!d) {
		return -1;
	}
	Window root = DefaultRootWindow(d);
	XSetWindowAttributes attrs;
	memset(&attrs, 0, sizeof attrs);
	attrs.override_redirect = True;
	attrs.event_mask = PropertyChangeMask | StructureNotifyMask;
	Window xw = XCreateWindow(d, root, -100, -100, 1, 1, 0, CopyFromParent, CopyFromParent,
		(Visual *)CopyFromParent, CWOverrideRedirect | CWEventMask, &attrs);
	Atom type = XInternAtom(d, "_NET_STARTUP_INFO_BEGIN", False);
	Atom more = XInternAtom(d, "_NET_STARTUP_INFO", False);

	for (int off = 0; off < len; off += 20) {
		XEvent ev;
		memset(&ev, 0, sizeof ev);
		ev.xclient.type = ClientMessage;
		ev.xclient.display = d;
		ev.xclient.window = xw;
		ev.xclient.message_type = type;
		ev.xclient.format = 8;
		int n = len - off < 20 ? len - off : 20;
		memcpy(ev.xclient.data.b, msg + off, n);
		XSendEvent(d, root, False, PropertyChangeMask, &ev);
		type = more;
	}
	XDestroyWindow(d, xw);
	XCloseDisplay(d); // заодно отправляет буфер
	return 0;
}
*/
import "C"

import "errors"

func broadcast(msg []byte) error {
	cmsg := C.CBytes(msg)
	defer C.free(cmsg)
	if C.zt_broadcast((*C.char)(cmsg), C.int(len(msg))) != 0 {
		return errors.New("startup notification: не удалось подключиться к X-серверу")
	}
	return nil
}
