// Read from the user agent: on Windows, Wails injects its environment only after the page starts loading.
export const mac = navigator.userAgent.includes('Macintosh');
export const windows = navigator.userAgent.includes('Windows NT');
export const desktop = mac || windows;
export const fileManager = windows ? 'File Explorer' : 'Finder';
