$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = "openmsx.exe" # Se não estiver no PATH, coloque o caminho completo
$psi.Arguments = "-control stdio"
$psi.UseShellExecute = $false
$psi.RedirectStandardInput = $true

$process = [System.Diagnostics.Process]::Start($psi)
$sw = $process.StandardInput

Write-Host "--- openMSX Bridge Ativa ---" -ForegroundColor Cyan
Write-Host "Digite o comando MSX (ex: set pause on) ou 'exit' para fechar."

while ($true) {
    $input = Read-Host "Comando"
    if ($input -eq "exit") { break }

    # Envia o comando no formato XML que o openMSX exige
    $xmlCommand = "<command>$input</command>"
    $sw.WriteLine($xmlCommand)
    Write-Host "Enviado: $xmlCommand" -ForegroundColor Gray
}

$process.Kill()