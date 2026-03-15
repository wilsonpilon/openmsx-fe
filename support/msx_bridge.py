import subprocess
import threading
import sys
import os

class OpenMSXBridge:
    def __init__(self, executable="openmsx.exe"):
        self.executable = executable
        self.process = None
        self._stop_event = threading.Event()
        self.on_output_received = None

    def is_running(self):
        if self.process is None:
            return False

        # O poll() retorna None se o processo ainda estiver rodando
        status = self.process.poll()
        if status is not None:
            # Se o processo terminou, limpamos a referência para não enganar outras threads
            self.process = None
            return False
        return True

    def start(self, extra_args=None):
        """Inicia o openMSX com redirecionamento de entrada/saída."""
        if self.is_running():
            self.stop()

        self._stop_event.clear()
        try:
            cmd = [self.executable, "-control", "stdio"]
            if extra_args:
                cmd.extend(extra_args)

            work_dir = os.path.dirname(self.executable) if os.path.isabs(self.executable) else None

            self.process = subprocess.Popen(
                cmd,
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                bufsize=1,
                cwd=work_dir,
                creationflags=subprocess.CREATE_NO_WINDOW if os.name == 'nt' else 0
            )

            threading.Thread(target=self._read_output, daemon=True).start()

            # Envia comandos de inicialização após 3 segundos (tempo para o openMSX carregar o core)
            boot_commands = ["set renderer sdlgl-pp", "set power on"]
            threading.Timer(3.0, lambda: self._send_boot_sequence(boot_commands)).start()

        except Exception as e:
            if self.on_output_received:
                self.on_output_received(f"Erro ao iniciar openMSX: {e}")

    def _send_boot_sequence(self, commands):
        """Envia uma sequência de comandos com pequeno intervalo entre eles"""
        for cmd in commands:
            if self.is_running():
                self.send_command(cmd)
                # Pequena pausa entre comandos para não sobrecarregar o buffer do openMSX
                threading.Event().wait(0.5)

    def send_command(self, command):
        """Envia um comando no formato XML esperado pelo openMSX."""
        if self.is_running() and self.process.stdin:
            xml_command = f"<command>{command}</command>\n"
            try:
                self.process.stdin.write(xml_command)
                self.process.stdin.flush()
            except OSError as e:
                if self.on_output_received:
                    self.on_output_received(f"Erro de I/O na Bridge: {e}")

    def _read_output(self):
        """Lê as respostas do openMSX."""
        while self.is_running() and not self._stop_event.is_set():
            line = self.process.stdout.readline()
            if not line:
                break

            if self.on_output_received:
                clean_line = line.replace("<reply>", "").replace("</reply>", "").strip()
                if clean_line:
                    self.on_output_received(clean_line)

    def stop(self):
        """Fecha o emulador graciosamente."""
        self._stop_event.set()
        if self.process:
            try:
                self.process.terminate()
                self.process.wait(timeout=2)
            except:
                try:
                    self.process.kill()
                except:
                    pass
            self.process = None