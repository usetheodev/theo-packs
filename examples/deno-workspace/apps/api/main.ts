import { Hono } from "hono";
const app = new Hono();
app.get("/", (c) => c.text("api"));
Deno.serve({ port: 8080 }, app.fetch);
