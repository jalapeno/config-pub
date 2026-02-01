FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/config-pub ./cmd/config-pub

FROM gcr.io/distroless/base-debian12:nonroot
COPY --from=build /out/config-pub /bin/config-pub
ENTRYPOINT ["/bin/config-pub"]

