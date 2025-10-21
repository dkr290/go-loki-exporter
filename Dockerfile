FROM golang:alpine3.21 as builder

WORKDIR /build
COPY go.mod go.sum ./
RUN ls -l
RUN go mod download && go mod tidy
COPY . .
RUN go build -o lokiexport .


# Input parameters for the Dockerfile expected in os.Getenv



FROM golang:alpine3.21
# Add maintainer info
LABEL maintainer="Danail Surudzhiyski"
RUN addgroup -S pipeline && adduser -S k8s-pipeline -G pipeline

WORKDIR /home/k8s-pipeline


COPY --from=builder /build/lokiexport .

RUN ls -l

USER k8s-pipeline



CMD ["./lokiexport"]
