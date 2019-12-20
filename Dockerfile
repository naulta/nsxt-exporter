  
FROM golang:alpine as builder
RUN mkdir /app 
ADD . /app/ 
WORKDIR /app 
RUN go build -o main . 

FROM alpine:edge
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/main .
EXPOSE 8080
ENV NSXTHOST="10.175.72.111"
ENV NSXTUSER="user"
ENV NSXTPASS="pass"
CMD ["/main"]