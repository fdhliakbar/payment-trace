FROM docker.io/oven/bun:1.1-alpine
WORKDIR /app
COPY package.json ./
RUN bun install
COPY index.ts ./
ENV PORT=8082
EXPOSE 8082
CMD ["bun", "run", "index.ts"]
