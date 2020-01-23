# Variables to compile go client
GO_CLIENT=bitwrk-client
GO_CLIENT_BUILD_DIR=./client/cmd/$(GO_CLIENT)

# Variables to zip addon and client daemon into one 
TMPDIR=tmp
ZIPNAME=render_bitwrk.zip
CLIENT_DIR=bitwrk_client
RESOURCE_DIR=resources
RENDER_DIR=bitwrk-blender
ADDON_DIR=render_bitwrk

all: zip-make

go-build:
	go build $(GO_CLIENT_BUILD_DIR)/$(CLIENT_EXEC)

go-clean:
	rm $(GO_CLIENT)

tmp-make:
	mkdir $(TMPDIR) && \
	cp -r $(RENDER_DIR)/$(ADDON_DIR) $(TMPDIR)/ && \
	mkdir $(TMPDIR)/$(ADDON_DIR)/$(CLIENT_DIR) && \
	cp $(GO_CLIENT) $(TMPDIR)/$(ADDON_DIR)/$(CLIENT_DIR)/ && \
	cp -r $(RESOURCE_DIR) $(TMPDIR)/$(ADDON_DIR)/$(CLIENT_DIR)/ 


tmp-clean:
	rm -rf $(TMPDIR)

zip-clean:
	rm $(ZIPNAME)

zip-make: go-build tmp-make
	@echo "Zip process starting"
	cd $(TMPDIR) && \
	zip -r ../$(ZIPNAME) *

clean: go-clean tmp-clean
