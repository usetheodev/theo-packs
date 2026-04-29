import { Hono } from "hono";

const app = new Hono();
app.get("/", (c) => c.text("ok"));

Deno.serve({ port: 8080 }, app.fetch);
