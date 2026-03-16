// Wails v2 runtime stub - replaced by Wails at build time
// During development with `wails dev`, the real runtime is injected

export function EventsOn(eventName, callback) {
  return window.runtime?.EventsOn(eventName, callback) || (() => {});
}

export function EventsOff(eventName) {
  window.runtime?.EventsOff(eventName);
}

export function EventsOnce(eventName, callback) {
  return window.runtime?.EventsOnce(eventName, callback) || (() => {});
}

export function EventsOnMultiple(eventName, callback, maxCallbacks) {
  return window.runtime?.EventsOnMultiple(eventName, callback, maxCallbacks) || (() => {});
}

export function EventsEmit(eventName, ...data) {
  window.runtime?.EventsEmit(eventName, ...data);
}

export function Quit() {
  window.runtime?.Quit();
}
