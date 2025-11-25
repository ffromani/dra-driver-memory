#!/bin/bash
exec kubectl wait --for=condition=Ready nodes --all --timeout=120s
