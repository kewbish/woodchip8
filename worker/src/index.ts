export interface Env {
  WOODCHIP: DurableObjectNamespace;
}

export { Woodchip } from "./woodchip";
export default {
  async fetch(
    request: Request,
    env: Env,
    ctx: ExecutionContext
  ): Promise<Response> {
    if (new URL(request.url).pathname == "/newWoodchip") {
      const id = env.WOODCHIP.newUniqueId();
      return new Response(id.toString(), {
        headers: { "Access-Control-Allow-Origin": "*" },
      });
    } else {
      const reqCopy = request.clone();
      const json = (await request.json()) as any;
      if (!json || !json["name"]) {
        return new Response("No Woodchip room name provided", { status: 400 });
      }
      const dObj = env.WOODCHIP.get(json["name"]);
      return dObj.fetch(request);
    }
  },
};
