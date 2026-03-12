# ping.ps1
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

$ADDRESS = "localhost:50051"
$SERVICE = "proto.v1.IngestorService"
$METHOD = "IngestEvent"

$jsonPayload = @'
{
  "event": {
    "event_id": "manual-ping-001",
    "event_type": "ping",
    "source": "powershell-client"
  }
}
'@

Write-Host "🚀 Sending event to $ADDRESS..." -ForegroundColor Cyan

$response = $jsonPayload | grpcurl -plaintext -d "@" $ADDRESS "$SERVICE/$METHOD"

if ($LASTEXITCODE -eq 0) {
    Write-Host "✅ Success!" -ForegroundColor Green
    $response
} else {
    Write-Host "❌ Error: grpcurl failed." -ForegroundColor Red
}