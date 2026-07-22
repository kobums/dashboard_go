tag=latest

all: server

server:
	go build -o bin/dashboard main.go

run:
	go run main.go

test:
	go test -v ./...

linux:
	env GOOS=linux GOARCH=amd64 go build -o bin/dashboard.linux main.go

dockerbuild:
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags '-s' -o bin/dashboard.linux main.go

web:
	cd ../dashboard_web && npm run build
	rm -rf dist && cp -r ../dashboard_web/dist dist

docker: dockerbuild web
	docker build --platform linux/amd64 -t kobums/dashboard:$(tag) .

dockerrun:
	docker run --env-file .env --platform linux/amd64 -d --name="dashboard" -p 8010:8010 kobums/dashboard:$(tag)

push: docker
	docker push kobums/dashboard:$(tag)

clean:
	rm -f bin/dashboard