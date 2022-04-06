MAGE:= go run mage.go -v

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

install:
	$(MAGE) Install

publish:
	$(MAGE) Publish

clean:
	$(MAGE) Clean
