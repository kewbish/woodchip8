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
    const id = env.WOODCHIP.idFromName("woodchip-main");
    const dObj = env.WOODCHIP.get(id);
    return dObj.fetch(request);
  },
};
