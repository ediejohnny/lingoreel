/**
 * keepDropsInTheApp stops the WebView from opening by itself whatever is dragged onto the window: a video would play
 * in a browser window of its own, without the subtitles, and a link or a page would load inside the app. A dropped
 * file still reaches the app through Wails, which hands its path to the backend.
 */
export function keepDropsInTheApp(target: EventTarget): void {
  const refuse = (event: Event) => event.preventDefault();
  target.addEventListener("dragover", refuse);
  target.addEventListener("drop", refuse);
}
