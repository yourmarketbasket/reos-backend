FROM public.ecr.aws/docker/library/golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o server ./cmd/server

FROM public.ecr.aws/docker/library/alpine:latest AS runner
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
RUN addgroup -S cosmic && adduser -S cosmic -G cosmic
COPY --from=builder /app/server .
RUN chown -R cosmic:cosmic /app
USER cosmic
EXPOSE 8080
CMD ["./server"]
