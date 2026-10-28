// Cloudflare Worker in front of the Go server container (see wrangler.jsonc).
import { Container, getContainer } from "@cloudflare/containers";

export class SaarthiContainer extends Container {
  defaultPort = 8080;
  sleepAfter = "30m"; // idle containers stop; the next request starts a fresh one
}

export default {
  fetch(request, env) {
    // Every request goes to the same instance, so all users share one database.
    return getContainer(env.SAARTHI).fetch(request);
  },
};
