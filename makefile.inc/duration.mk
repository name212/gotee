define NOW_MICROSECONDS
date +"%s%6N"
endef

HUMAN_DURATION_MICROSECONDS= sh -c ' \
start="$$1"; \
end="$$2"; \
duration_micros="$$((end - start))"; \
hours=$$(( duration_micros / 3600000000 )); \
minutes=$$(( (duration_micros / 60000000) % 60 )); \
seconds=$$(( (duration_micros / 1000000) % 60 )); \
micros=$$(( duration_micros % 1000000 )); \
out="$$(printf "%02d.%06ds" $$seconds $$micros)"; \
if [ "$$hours" -gt 0 ]; then \
  out="$$(printf "%02dh %02dm" $$hours $$minutes) $$out"; \
elif [ "$$minutes" -gt 0 ]; then \
  out="$$(printf "%02dm" $$minutes) $$out"; \
fi; \
echo -n "$$out"'