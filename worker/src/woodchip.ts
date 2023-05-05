import { Stack } from "stack-typescript";

const HEADERS = { headers: { "Access-Control-Allow-Origin": "*" } };

export class Woodchip implements DurableObject {
  memory: Int8Array = new Int8Array(4096);
  pc: number = 0x200;
  index: number = 0;
  regs: Int8Array = new Int8Array(16);
  stack: Stack<number> = new Stack<number>();
  delayTimer: number = 0;
  soundTimer: number = 0;
  shouldQuit: boolean = false;

  state: DurableObjectState;
  sessions: WebSocket[];

  constructor(state: DurableObjectState, env: any) {
    this.state = state;
    this.sessions = [];
  }

  async fetch(request: Request) {
    const url = new URL(request.url);
    const newReq = request.clone();
    const json = await request.json();

    if (url.pathname == "/websocket") {
      if (request.headers.get("Upgrade") != "websocket") {
        return new Response("Expected websocket upgrade request", {
          status: 400,
        });
      }
      const pair = new WebSocketPair();
      await this.handleSession(pair[1], newReq, url, json);
      return new Response(null, { status: 101, webSocket: pair[0] });
    } else {
      return new Response(
        "Woodchip uses websockets, try initializing websocket request",
        { status: 404 }
      );
    }
  }

  async handleSession(
    websocket: WebSocket,
    request: Request,
    url: URL,
    json: any
  ) {
    let memory: Int8Array =
      (await this.state.storage.get("memory")) || new Int8Array(4096);
    let pc: number = (await this.state.storage.get("pc")) || 0x200;
    let index: number = (await this.state.storage.get("index")) || 0;
    let regs: Int8Array =
      (await this.state.storage.get("regs")) || new Int8Array(16);
    let stack: number[] = (await this.state.storage.get("stack")) || [];
    let delayTimer: number = (await this.state.storage.get("delayTimer")) || 0;
    let soundTimer: number = (await this.state.storage.get("soundTimer")) || 0;
    let shouldQuit: boolean =
      (await this.state.storage.get("shouldQuit")) || false;

    let ready = false;
    websocket.accept();
    this.sessions.push(websocket);
    websocket.addEventListener("message", async (msg: any) => {
      try {
        const data = JSON.parse(msg.data);
        if (!ready) {
          websocket.send(JSON.stringify({ ready: true }));
          ready = true;
        }

        switch (data.commandPath) {
          case "/setMemory":
            await this.state.storage.put(
              "memory",
              (json as { memory: Int8Array })["memory"]
            );
            this.broadcast(
              JSON.stringify({
                memory: (json as { memory: Int8Array })["memory"],
              })
            );
            break;
          case "/setPC":
            await this.state.storage.put("pc", (json as { pc: number })["pc"]);
            this.broadcast(
              JSON.stringify({
                pc: (json as { pc: number })["pc"],
              })
            );
            break;
          case "/setIndex":
            await this.state.storage.put(
              "index",
              (json as { index: number })["index"]
            );
            this.broadcast(
              JSON.stringify({
                index: (json as { index: number })["index"],
              })
            );
            break;
          case "/resetRegs":
            await this.state.storage.put(
              "regs",
              [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
            );
            this.broadcast(
              JSON.stringify({
                regs: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0],
              })
            );
            break;
          case "/setReg":
            let newRegs: Int8Array =
              (await this.state.storage.get("regs")) || new Int8Array(16);
            newRegs[(json as { regIndex: number; value: number }).regIndex] = (
              json as { regIndex: number; value: number }
            ).value;
            await this.state.storage.put("regs", newRegs);
            this.broadcast(JSON.stringify({ regs: newRegs }));
            break;
          case "/setStack":
            await this.state.storage.put(
              "stack",
              (json as { stack: number[] })["stack"]
            );
            this.broadcast(
              JSON.stringify({ stack: (json as { stack: number[] })["stack"] })
            );
            break;
          case "/setDelayTimer":
            await this.state.storage.put(
              "delayTimer",
              (json as { delayTimer: number })["delayTimer"]
            );
            this.broadcast(
              JSON.stringify({
                delayTimer: (json as { delayTimer: number })["delayTimer"],
              })
            );
            break;
          case "/setSoundTimer":
            await this.state.storage.put(
              "soundTimer",
              (json as { soundTimer: number })["soundTimer"]
            );
            this.broadcast(
              JSON.stringify({
                soundTimer: (json as { soundTimer: number })["soundTimer"],
              })
            );
            break;
          case "/setShouldQuit":
            await this.state.storage.put(
              "shouldQuit",
              (json as { shouldQuit: boolean })["shouldQuit"]
            );
            this.broadcast(
              JSON.stringify({
                shouldQuit: (json as { shouldQuit: boolean })["shouldQuit"],
              })
            );
            break;
          case "/getMemory":
            websocket.send(JSON.stringify({ memory }));
          case "/getPC":
            websocket.send(JSON.stringify({ pc }));
          case "/getIndex":
            websocket.send(JSON.stringify({ index }));
          case "/getRegs":
            websocket.send(JSON.stringify({ regs }));
          case "/getStack":
            websocket.send(JSON.stringify({ stack }));
          case "/getDelayTimer":
            websocket.send(JSON.stringify({ delayTimer }));
          case "/getSoundTimer":
            websocket.send(JSON.stringify({ soundTimer }));
          case "/getShouldQuit":
            websocket.send(JSON.stringify({ shouldQuit }));
        }
      } catch (err: any) {
        websocket.send(JSON.stringify({ error: err.message }));
      }
    });
  }

  broadcast(message: string) {
    this.sessions.forEach((session) => {
      try {
        session.send(message);
        return true;
      } catch (err) {
        return;
      }
    });
  }
}
