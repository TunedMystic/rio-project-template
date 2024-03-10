# ----------------------------------------------------
# In stage one, we're installing dependencies,
# building the golang application
# and compressing the app binary.
# ----------------------------------------------------

FROM golang:1.22.1-alpine3.19 as builder

ENV GO111MODULE=on \
    GOOS=linux \
    CGO_ENABLED=0

ARG BUILD_HASH

WORKDIR /build

RUN apk add --no-cache upx

COPY . .
RUN go build -ldflags="-s -w -X 'main.BuildHash=$BUILD_HASH' -X 'main.BuildDate=$(date)'" -o app ./...
RUN upx /build/app


# ----------------------------------------------------
# In stage two, we're copying the app binary
# into a minimal image.
# ----------------------------------------------------

FROM scratch as final

WORKDIR /x

COPY --from=builder /build/app .

EXPOSE 3000

CMD ["/x/app"]
