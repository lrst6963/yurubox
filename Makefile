.PHONY: build clean web

APP_NAME = phonecall

all: build

web:
	@echo "构建Web前端..."
	cd web && npm install && npm run build
	@echo "Web前端构建完成."

build: web
	@echo "构建$(APP_NAME)..."
	CGO_ENABLED=0 go build -ldflags "-w -s" -o $(APP_NAME) .
	@echo "构建完成. 可以运行: ./$(APP_NAME)"

clean:
	@echo "清理..."
	rm -f $(APP_NAME)
	#rm -f cert.pem key.pem
	rm -rf public/
	@echo "清理完成."
