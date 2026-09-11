FROM docker.io/library/golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY main.go ./
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /out/order-go .

FROM docker.io/library/alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/order-go /usr/local/bin/order-go
ENV PORT=8080
EXPOSE 8080
CMD ["order-go"]
