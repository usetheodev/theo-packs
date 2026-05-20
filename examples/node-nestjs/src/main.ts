// Minimal NestJS example — mirrors theo-stacks/templates/node-nestjs.
// Used by theo-packs as fixture: validates that the Node provider
// honors a `tsc` build step + `node dist/main.js` start command.
import "reflect-metadata";

async function bootstrap() {
  console.log("NestJS example booting");
}
bootstrap();
