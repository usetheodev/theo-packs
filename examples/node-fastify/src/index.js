const Fastify = require("fastify");
const fastify = Fastify();
fastify.get("/health", async () => ({ status: "ok" }));
fastify.listen({ host: "0.0.0.0", port: process.env.PORT || 3000 });
