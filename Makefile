MAGE:= go run mage.go -v

.PHONY: build
build:
	$(MAGE) Build

test:
	$(MAGE) Test

test-unit:
	$(MAGE) TestUnit

test-integration:
	$(MAGE) TestIntegration

test-local-integration:
	$(MAGE) TestLocalIntegration

#TODO: add install target
install:
	$(MAGE) Install

publish:
	$(MAGE) Publish

clean:
	$(MAGE) Clean
