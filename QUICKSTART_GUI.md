# Quickstart (Windows)

## 1) Install Go
Install a recent Go version and ensure `go version` works in PowerShell.

## 2) Build the parser
Open PowerShell in the project folder and run:

```powershell
go mod download
go build -o acars_parser.exe .\cmd\acars_parser
```

## 3) Run extract (CLI)
```powershell
.\acars_parser.exe extract -input messages.jsonl -output out.json -pretty -all
```

## 4) Run GUI wrapper
```powershell
python .\gui\acars_parser_gui.py
```

Notes:
- Input must be JSONL (one JSON object per line) matching either:
  - `internal/acars.Message`, or
  - NATS wrapper (`internal/acars.NATSWrapper`) with `message{...}` inside.
