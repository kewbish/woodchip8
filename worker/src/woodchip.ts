import { Stack } from "stack-typescript";

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

  constructor(state: DurableObjectState, env: any) {
    this.state = state;
  }

  async fetch(request: Request) {
    const url = new URL(request.url);
    const json = await request.json();

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

    if (request.method == "POST") {
      switch (url.pathname) {
        case "/setMemory":
          await this.state.storage.put(
            "memory",
            (json as { memory: Int8Array })["memory"]
          );
          break;
        case "/setPC":
          await this.state.storage.put("pc", (json as { pc: number })["pc"]);
          break;
        case "/setIndex":
          await this.state.storage.put(
            "index",
            (json as { index: number })["index"]
          );
          break;
        case "/resetRegs":
          await this.state.storage.put(
            "regs",
            [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
          );
          break;
        case "/setReg":
          let newRegs = regs;
          newRegs[(json as { regIndex: number; value: number }).regIndex] = (
            json as { regIndex: number; value: number }
          ).value;
          await this.state.storage.put("regs", newRegs);
          break;
        case "/setStack":
          await this.state.storage.put(
            "stack",
            (json as { stack: number[] })["stack"]
          );
          break;
        case "/setDelayTimer":
          await this.state.storage.put(
            "delayTimer",
            (json as { delayTimer: number })["delayTimer"]
          );
          break;
        case "/setSoundTimer":
          await this.state.storage.put(
            "soundTimer",
            (json as { soundTimer: number })["soundTimer"]
          );
          break;
        case "/setShouldQuit":
          await this.state.storage.put(
            "shouldQuit",
            (json as { shouldQuit: boolean })["shouldQuit"]
          );
          break;
        default:
          return new Response("Unknown path", { status: 404 });
      }
    } else {
      switch (url.pathname) {
        case "/getMemory":
          return new Response(JSON.stringify({ memory }));
        case "/getPC":
          return new Response(JSON.stringify({ pc }));
        case "/getIndex":
          return new Response(JSON.stringify({ index }));
        case "/getRegs":
          return new Response(JSON.stringify({ regs }));
        case "/getStack":
          return new Response(JSON.stringify({ stack }));
        case "/getDelayTimer":
          return new Response(JSON.stringify({ delayTimer }));
        case "/getSoundTimer":
          return new Response(JSON.stringify({ soundTimer }));
        case "/getShouldQuit":
          return new Response(JSON.stringify({ shouldQuit }));
        default:
          return new Response("Unknown path", { status: 404 });
      }
    }
    return new Response("Unknown path", { status: 404 });
  }
}
