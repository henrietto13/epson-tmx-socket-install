package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// socketFileContent Contiene la configuración de la unidad de socket systemd
// Escucha en todas las interfaces de red en el puerto TCP 9100.
const socketFileContent = `[Unit]
Description=ESC/POS Printer Socket

[Socket]
ListenStream=0.0.0.0:9100
Accept=yes

[Install]
WantedBy=sockets.target
`

// serviceFileContent Crea la configuración de la unidad de servicio, con el path de la impresora.
// Este es un servicio de plantilla que se instancia para cada conexión entrante.
// Utiliza 'tee' para canalizar los datos entrantes a la impresora y /dev/null
// Se canaliza a /dev/null para darle unos microsegundos a la impresora y detectar la impresion
func serviceFileContent(printerPath string) string {
	return fmt.Sprintf(`[Unit]
Description=ESC/POS Printer Service

[Service]
ExecStart=-/usr/bin/tee /dev/null > %s
StandardInput=socket
`, printerPath)
}

// findPrinters Busca dispositivos de impresora en /dev/usb y devuelve una lista.
// Se espera que los dispositivos sigan el patrón /dev/usb/lpX.
func findPrinters() ([]string, error) {
	// Busca archivos que coincidan con el patrón /dev/usb/lp*
	matches, err := filepath.Glob("/dev/usb/lp*")
	if err != nil {
		return nil, fmt.Errorf("error al buscar impresoras: %w", err)
	}

	// Filtra los resultados para incluir solo los dispositivos de caracteres
	var printers []string
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue // Ignora los errores al obtener información del archivo
		}
		// S_IFCHR representa un dispositivo de caracteres, como una impresora
		if info.Mode()&os.ModeCharDevice != 0 {
			printers = append(printers, match)
		}
	}

	return printers, nil
}

// selectPrinter muestra una lista de impresoras y solicita al usuario que elija una.
func selectPrinter(printers []string) (string, error) {
	if len(printers) == 0 {
		return "", fmt.Errorf("no se encontraron impresoras USB en /dev/usb/lpX")
	}

	fmt.Println("\nSe encontraron las siguientes impresoras USB:")
	for i, p := range printers {
		fmt.Printf("%d. %s\n", i+1, p)
	}

	var choice int
	for {
		fmt.Print("Por favor, selecciona el número de la impresora que deseas usar: ")
		_, err := fmt.Scanln(&choice)
		if err != nil || choice < 1 || choice > len(printers) {
			fmt.Println("Entrada inválida. Por favor, ingresa un número de la lista.")
			continue
		}
		break
	}

	return printers[choice-1], nil
}

func main() {
	fmt.Println("Iniciando la configuración del servicio de impresora ESC/POS...")

	// --- Paso 1: Checar acceso root ---
	// Necesitamos escribir archivos en /etc/systemd/system y ejecutar comandos systemctl,
	// que requieren permisos elevados.
	if os.Geteuid() != 0 {
		log.Fatal("Este programa debe ejecutarse como root o con sudo.")
	}
	fmt.Println("✓ Permisos de root confirmados.")

	// --- Paso 2: Encontrar y seleccionar la impresora ---
	printers, err := findPrinters()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Println(len(printers))
	for _, printer := range printers {
		fmt.Println(printer)
	}

	selectedPrinter, err := selectPrinter(printers)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Printf("✓ Impresora seleccionada: %s\n", selectedPrinter)

	// --- Paso 3: Definir rutas de archivos ---
	socketFilePath := "/etc/systemd/system/escpos-printer.socket"
	serviceFilePath := "/etc/systemd/system/escpos-printer@.service"

	// --- Paso 4: Escribe los archivos de unidad systemd ---
	err = os.WriteFile(socketFilePath, []byte(socketFileContent), 0644)
	if err != nil {
		log.Fatalf("Error al escribir el archivo de socket: %v", err)
	}
	fmt.Printf("✓ Archivo de socket creado exitosamente: %s\n", socketFilePath)

	// Genera el contenido del servicio con la ruta de la impresora seleccionada
	serviceContent := serviceFileContent(selectedPrinter)
	err = os.WriteFile(serviceFilePath, []byte(serviceContent), 0644)
	if err != nil {
		log.Fatalf("Error al escribir el archivo de servicio: %v", err)
	}
	fmt.Printf("✓ Archivo de servicio creado exitosamente: %s\n", serviceFilePath)

	// --- Paso 5: Ejecuta los comandos systemctl para habilitar e iniciar el servicio ---
	// Habilita el socket para que se inicie durante el arranque y lo inicia inmediatamente.
	commands := [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "enable", "--now", "escpos-printer.socket"},
		{"systemctl", "restart", "escpos-printer.socket"},
	}

	for _, cmdArgs := range commands {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		fmt.Printf("Ejecutando: %s...\n", strings.Join(cmd.Args, " "))
		output, err := cmd.CombinedOutput() // CombinedOutput obtiene tanto stdout como stderr
		if err != nil {
			log.Fatalf("Error al ejecutar el comando '%s': %v\nSalida: %s", strings.Join(cmd.Args, " "), err, string(output))
		}
		fmt.Printf("✓ Comando exitoso.\n")
	}

	fmt.Println("\n🎉 ¡Configuración completa! El socket de la impresora ESC/POS está activo y habilitado.")
	fmt.Println("La PC está lista para aceptar trabajos de impresión en el puerto TCP 9100.")
}
