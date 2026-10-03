import { System } from '@wailsio/runtime';

export const mac = System.IsMac();
export const windows = System.IsWindows();
export const desktop = mac || windows;
export const fileManager = windows ? 'File Explorer' : 'Finder';
