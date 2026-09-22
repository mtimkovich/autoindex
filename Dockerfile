FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
COPY web/ ./web/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /autoindex .

FROM scratch
COPY --from=build /autoindex /autoindex
EXPOSE 8080
ENTRYPOINT ["/autoindex", "-root", "/data"]
