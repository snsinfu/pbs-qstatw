OUTDIR = out
PRODUCT = $(OUTDIR)/qstatw
ARTIFACTS = $(PRODUCT) $(OUTDIR)

.PHONY: all clean

all: $(PRODUCT)
	@:

clean:
	rm -rf $(ARTIFACTS)

$(PRODUCT): qstatw/cmd/main.go
	mkdir -p $(OUTDIR)
	go build -o $@ ./qstatw/cmd
