  
FROM golang:alpine as builder
<<<<<<< HEAD
WORKDIR /main
RUN apk add --no-cache git
COPY . /main
RUN CGO_ENABLED=0 GOOS=linux go build 

FROM alpine:edge
RUN apk add --no-cache ca-certificates
COPY --from=builder /main/main /main
EXPOSE 8080
=======
RUN mkdir /app 
ADD . /app/ 
WORKDIR /app 
RUN go build -o main . 

#FROM golang:alpine as builder
#WORKDIR /main
#RUN apk add --no-cache git
#COPY . /main
#RUN CGO_ENABLED=0 GOOS=linux go build -o main

FROM alpine:edge
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/main .
EXPOSE 8080
ENV NSXTHOST="172.28.110.233"
>>>>>>> 963e531... fixing docker build to a lighter image
CMD ["/main"]