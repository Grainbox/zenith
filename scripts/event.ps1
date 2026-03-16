# ping.ps1
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

$ADDRESS = "localhost:8080"
$SERVICE = "proto.v1.IngestorService"
$METHOD = "IngestEvent"

$jsonPayload = @'
{
  	"event": {
		"event_id": "evt-level2-001",
		"event_type": "payment.completed",
		"source": "payment-service",
		"payload": "eyJhbW91bnQiOjI1MCwiY3VycmVuY3kiOiJVU0QifQ=="
	}
}
'@

Write-Host "🚀 Sending event to $ADDRESS..." -ForegroundColor Cyan

$response = $jsonPayload | grpcurl -plaintext -d "@" $ADDRESS "$SERVICE/$METHOD"

if ($LASTEXITCODE -eq 0) {
	Write-Host "✅ Success!" -ForegroundColor Green
	$response
}
else {
	Write-Host "❌ Error: grpcurl failed." -ForegroundColor Red
}