import http from 'node:http'
const port = Number(process.env.PORT ?? 3001)
http.createServer((_, res) => { res.end('hello from @scoped/web\n') }).listen(port)
