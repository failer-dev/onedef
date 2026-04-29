BACKEND_DIR := backend
ADDR ?= :8280
SDK_OUT := sdk
IR_JSON := $(SDK_OUT)/onedef.spec.json
IR_TMP := .onedef.spec.tmp.json
SDK_GEN_DIR := ../../onedef_dart/sdk_gen
SDK_CORE_PATH := ../../../onedef_dart/sdk_core

.PHONY: run-server generate generate-ir generate-sdk check clean

run-server:
	cd $(BACKEND_DIR) && go run ./cmd/server $(ADDR)

generate: generate-sdk

generate-ir:
	mkdir -p $(SDK_OUT)
	cd $(BACKEND_DIR) && go run ./cmd/ir > ../$(IR_JSON)

generate-sdk: generate-ir
	cp $(IR_JSON) $(IR_TMP)
	cd $(SDK_GEN_DIR) && dart run bin/onedef_dart_sdk_gen.dart generate \
		--input ../../example/chat/$(IR_TMP) \
		--out ../../example/chat/$(SDK_OUT) \
		--package-name chat_api \
		--core-path $(SDK_CORE_PATH)
	cp $(IR_TMP) $(IR_JSON)
	rm -f $(IR_TMP)

check:
	cd $(BACKEND_DIR) && go test ./...
	$(MAKE) generate

clean:
	rm -rf $(SDK_OUT) $(IR_TMP)
