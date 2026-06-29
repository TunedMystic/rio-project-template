# ----------------------------------------------------
# In stage one, we're installing dependencies,
# building the golang application
# and compressing the app binary.
# ----------------------------------------------------

FROM golang:1.26-alpine AS builder

ENV GO111MODULE=on \
    GOOS=linux \
    CGO_ENABLED=0

ARG BUILD_HASH

WORKDIR /build

RUN apk add --no-cache upx

COPY . .
RUN go build -mod=vendor \
    -ldflags="-s -w -X 'main.BuildHash=$BUILD_HASH' -X 'main.BuildDate=$(date)'" \
    -o app .
RUN upx /build/app


# ----------------------------------------------------
# In stage two, we're copying the app binary
# into a minimal image.
# ----------------------------------------------------

FROM scratch AS final

WORKDIR /x

COPY --from=builder /build/app .

# SQLite database directory (mount a volume here to persist:
#   docker run -v ./data:/data ...). DB file is /data/<ProjectName>.db.
ENV DB_DIR=/data
# Auth/email (set at runtime): APP_SECRET (required in prod), BASE_URL,
# POSTMARK_TOKEN, EMAIL_FROM, GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET.
# Billing (set at runtime): STRIPE_SECRET_KEY, STRIPE_WEBHOOK_SECRET,
# STRIPE_PRICE_PRO, STRIPE_PRICE_EBOOK.
EXPOSE 3000

CMD ["/x/app"]
