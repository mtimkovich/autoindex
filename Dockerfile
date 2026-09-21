FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go index.html ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /autoindex .

FROM scratch
COPY --from=build /autoindex /autoindex
EXPOSE 8080
ENTRYPOINT ["/autoindex", "-tree", "-root", "/data", "-addr", "8080"]
# Extra flags (e.g. -tree, -all) can be appended: docker run ... image -tree
