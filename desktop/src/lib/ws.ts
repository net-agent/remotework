import type { WsEvent } from "./types";

type EventHandler = (event: WsEvent) => void;

const RECONNECT_BASE = 1000;
const RECONNECT_MAX = 30000;

export class AgentWebSocket {
  private ws: WebSocket | null = null;
  private url = "";
  private handler: EventHandler;
  private reconnectDelay = RECONNECT_BASE;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private closed = false;

  constructor(handler: EventHandler) {
    this.handler = handler;
  }

  connect(apiBaseUrl: string) {
    this.closed = false;
    this.url = apiBaseUrl.replace(/^http/, "ws") + "/api/v1/ws";
    this.doConnect();
  }

  disconnect() {
    this.closed = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  get connected() {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  private doConnect() {
    if (this.closed) return;
    try {
      this.ws = new WebSocket(this.url);
    } catch {
      this.scheduleReconnect();
      return;
    }

    this.ws.onopen = () => {
      this.reconnectDelay = RECONNECT_BASE;
      // Subscribe to all events
      this.ws?.send(
        JSON.stringify({
          action: "subscribe",
          events: ["network.state", "service.status", "stream.update", "hub.stopped"],
        })
      );
    };

    this.ws.onmessage = (ev) => {
      try {
        const event = JSON.parse(ev.data) as WsEvent;
        this.handler(event);
      } catch {
        // ignore malformed messages
      }
    };

    this.ws.onclose = () => {
      this.ws = null;
      this.scheduleReconnect();
    };

    this.ws.onerror = () => {
      this.ws?.close();
    };
  }

  private scheduleReconnect() {
    if (this.closed) return;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.doConnect();
    }, this.reconnectDelay);
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, RECONNECT_MAX);
  }
}
