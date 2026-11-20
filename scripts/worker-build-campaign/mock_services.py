#!/usr/bin/env python3
"""Isolated test-only credential endpoint and streaming fake S3 endpoint."""
import http.server, json, os, ssl, threading, time

class Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = 'HTTP/1.1'
    def log_message(self, fmt, *args):
        print('%s %s' % (self.log_date_time_string(), fmt % args), flush=True)
    def respond(self, status, body=b'', headers=None):
        self.send_response(status)
        self.send_header('Content-Length', str(len(body)))
        for k, v in (headers or {}).items(): self.send_header(k, v)
        self.end_headers()
        if body: self.wfile.write(body)
    def do_POST(self):
        if self.server.server_port == 8000 and self.path.startswith('/internal/deployments/') and self.path.endswith('/clone-credential'):
            self.respond(200, b'{"username":"x-access-token","token":"public-repository-test-only"}', {'Content-Type':'application/json'})
        else: self.respond(404)
    def do_PUT(self):
        if self.server.server_port != 443:
            self.respond(404); return
        count=0
        try:
            if 'chunked' in self.headers.get('Transfer-Encoding','').lower():
                while True:
                    line=self.rfile.readline(65536)
                    if not line: raise EOFError('chunk header missing')
                    n=int(line.strip().split(b';')[0],16)
                    if n==0:
                        while self.rfile.readline(65536).strip(): pass
                        break
                    remaining=n
                    while remaining:
                        data=self.rfile.read(min(1048576,remaining))
                        if not data: raise EOFError('chunk data missing')
                        count+=len(data); remaining-=len(data)
                    if self.rfile.read(2)!=b'\r\n': raise ValueError('bad chunk terminator')
            else:
                remaining=int(self.headers.get('Content-Length','0'))
                while remaining:
                    data=self.rfile.read(min(1048576,remaining))
                    if not data: raise EOFError('body incomplete')
                    count+=len(data); remaining-=len(data)
            print('TEST_UPLOAD path=%s bytes=%d' % (self.path,count),flush=True)
            self.respond(200, headers={'ETag':'"test-only-etag"'})
        except Exception as e:
            print('TEST_UPLOAD_ERROR %r'%e, flush=True)
            self.respond(500)

class Server(http.server.ThreadingHTTPServer):
    daemon_threads=True

http=Server(('0.0.0.0',8000),Handler)
# AWS uses default HTTPS port 443.
https=Server(('0.0.0.0',443),Handler)
ctx=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
ctx.load_cert_chain('/e2e/mock.crt','/e2e/mock.key')
https.socket=ctx.wrap_socket(https.socket,server_side=True)
threading.Thread(target=http.serve_forever,daemon=True).start()
print('test mock services ready',flush=True)
https.serve_forever()
