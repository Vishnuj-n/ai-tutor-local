export {};

declare global {
  interface Window {
    go: any;
    [key: string]: any;
  }
}
