FROM docker.io/library/node:24-alpine
WORKDIR /app
COPY . .
WORKDIR /app/.
RUN npm install --ignore-scripts --package-lock=false && npm run build
CMD ["sh", "-c", "npm start -- --hostname 0.0.0.0 --port 3000"]
