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
      const queryParams = new URLSearchParams(
        new URL(request.url).searchParams
      );
      const name = queryParams.get("name");
      if (!name) {
        return new Response("No Woodchip room name provided", { status: 400 });
      }
      const dObj = env.WOODCHIP.get(env.WOODCHIP.idFromString(name));
      return dObj.fetch(request);
    }
  },
};
