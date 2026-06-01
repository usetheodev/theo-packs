import http from 'node:http'
const port = Number(process.env.PORT ?? 3000)
http.createServer((_, res) => { res.end('hello from @scoped/api\n') }).listen(port)
