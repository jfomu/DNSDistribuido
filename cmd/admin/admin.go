package main

import (
	"log"
	"context"
	"bufio"
	"fmt"
	"os"
	"strings"

	pb "github.com/jfomu/DNSDistribuido/internal/proto"
	"github.com/jfomu/DNSDistribuido/internal/config"
	"github.com/jfomu/DNSDistribuido/internal/nodo"
	//"google.golang.org/grpc"
)

//// ESTRUCTURAS
type RegistroCambio struct {
	Reloj []int32
	IP string
	Port string
}


//// VARIABLES GLOBALES
var configuracion *config.Config
var dominioRegistro map[string]*RegistroCambio // Almacena para cada dominio la información del último cambio

//// FUNCIONES

func separarNombreDominio(nombreDominio string) (string, string) {
	split := strings.Split(nombreDominio, ".")
	var nombre string
	var dominio string

	if len(split) == 2{
	nombre = split[0]
	dominio = split[1]
	} else {
		log.Fatalf("[ERROR] Error dividiendo la variable NombreDominio")
	}
	return nombre, dominio
}


func main() {
	log.Printf("= INICIANDO ADMIN =\n")

	// Cargar archivo de configuración
	log.Println("Cargando archivo de configuración")
	configuracion = config.GenConfig("config.json")

	// Inicializar variables
	log.Println("Inicializando variables")
	dominioRegistro = make(map[string]*RegistroCambio)
	
	log.Println("Estableciendo conexión con el Broker")
	conn, err := nodo.ConectarNodo(configuracion.Broker.Ip, configuracion.Broker.Port)
	if err != nil {
		log.Fatalf("Error al intentar conectar con el Broker: %s", err)
	}
	broker := pb.NewServicioNodoClient(conn)

	_, err = broker.ObtenerEstado(context.Background(), new(pb.Consulta))
	if err != nil {
		log.Fatalf("Error al llamar a ObtenerEstado(): %s", err)
	}


	// Recibir comando por la terminal
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("-> ")
		text, _ := reader.ReadString('\n')
		text = strings.Replace(text, "\n", "", -1)
		text = strings.ToLower(text)
		words := strings.Split(text, " ")
		
		//// Comando CREATE
		if strings.Compare("create", words[0]) == 0{ 
			// Verificar el número de parámetros y puntos en el nombre de dominio
			if len(words) != 3 ||  len(strings.Split(words[1], ".")) != 2 {
				log.Printf("[ERROR] Usar:\n\t create <nombre>.<dominio> <IP>\n")
				continue
			}
			
			
			_, dominio := separarNombreDominio(words[1])
			var ipDNS string
			var portDNS string
			var registroCambio *RegistroCambio

			
			// Verificar si hay que solicitar un servidor DNS al broker o usar el registrado
			if _, ok := dominioRegistro[dominio]; ok { // Si el registro está en memoria
				registroCambio = dominioRegistro[dominio]
				ipDNS = registroCambio.IP
				portDNS = registroCambio.Port
			} else { // Si el registro no está en memoria
				// Solicitar un servidor DNS aleatorio al Broker
				resp, err := broker.Get(context.Background(), new(pb.Consulta))
				if err != nil {
				log.Fatalf("Error al llamar a Get(): %s", err)
				}
				ipDNS = resp.Ip
				portDNS = resp.Port

				// Iniciar el registro en memoria
				registroCambio = new(RegistroCambio)
			}
			
			// Conectar al servidor DNS
			log.Println("Estableciendo conexión con el nodo DNS")	
			conn, err := nodo.ConectarNodo(ipDNS, portDNS)
			if err != nil {
				log.Fatalf("Error al intentar conectar al servidor DNS: %s", err)
			}
			dns := pb.NewServicioNodoClient(conn)

			// Generar la consulta y enviarla
			consulta := new(pb.Consulta)
			consulta.NombreDominio = words[1]
			consulta.Ip = words[2]
			dnsResp, err := dns.Create(context.Background(), consulta)
			if err != nil {
				log.Printf("Error al llamar a Create(): %s", err)
				continue
				}
			log.Printf("Create exitoso! - Reloj: %+v", dnsResp.Reloj)

			// Actualizar la información del reloj en el registro
			registroCambio.Reloj = dnsResp.Reloj
			registroCambio.IP = dnsResp.Ip
			registroCambio.Port = dnsResp.Port
			dominioRegistro[dominio] = registroCambio
			
			


		//// Comando UPDATE
		} else if strings.Compare("update", words[0]) == 0 {
			// Verificar el número de parámetros y puntos en el nombre de dominio
			if len(words) != 4 ||  len(strings.Split(words[1], ".")) != 2 || (words[2] != "ip" && words[2] != "name") { 
				log.Printf("[ERROR] Usar:\n\t update <nombre>.<dominio> <opcion> <parámetro>\n\t <opcion> puede tomar los valores de ip o name\n")
				continue
			}
			
			_, dominio := separarNombreDominio(words[1])
			var ipDNS string
			var portDNS string
			var registroCambio *RegistroCambio

			// Verificar si hay que solicitar un servidor DNS al broker o usar el registrado
			if _, ok := dominioRegistro[dominio]; ok { // Si el registro está en memoria
				registroCambio = dominioRegistro[dominio]
				ipDNS = registroCambio.IP
				portDNS = registroCambio.Port
			} else { // Si el registro no está en memoria
				// Solicitar un servidor DNS aleatorio al Broker
				resp, err := broker.Get(context.Background(), new(pb.Consulta))
				if err != nil {
				log.Fatalf("Error al llamar a Get(): %s", err)
				}
				ipDNS = resp.Ip
				portDNS = resp.Port

				// Iniciar el registro en memoria
				registroCambio = new(RegistroCambio)
			}

			// Conectar al servidor DNS
			log.Println("Estableciendo conexión con el nodo DNS")
			conn, err := nodo.ConectarNodo(ipDNS, portDNS)
			if err != nil {
				log.Fatalf("Error al intentar conectar al servidor DNS: %s", err)
			}
			dns := pb.NewServicioNodoClient(conn)

			consulta := new(pb.ConsultaUpdate)
			consulta.NombreDominio = words[1]
			consulta.Opcion = words[2]
			consulta.Param = words[3]

			dnsResp, err := dns.Update(context.Background(), consulta)
			if err != nil {
				log.Printf("Error al llamar a Update(): %s", err)
				continue
				}
			log.Printf("Update exitoso! - Reloj: %+v", dnsResp.Reloj)
			

		//// Comando DELETE
		} else if strings.Compare("delete", words[0]) == 0 {
			// Verificar el número de parámetros y puntos en el nombre de dominio
			if len(words) != 2 ||  len(strings.Split(words[1], ".")) != 2 {
				log.Printf("[ERROR] Usar:\n\t delete <nombre>.<dominio>\n")
				continue
			}

			_, dominio := separarNombreDominio(words[1])
			var ipDNS string
			var portDNS string
			var registroCambio *RegistroCambio

			// Verificar si hay que solicitar un servidor DNS al broker o usar el registrado
			if _, ok := dominioRegistro[dominio]; ok { // Si el registro está en memoria
				registroCambio = dominioRegistro[dominio]
				ipDNS = registroCambio.IP
				portDNS = registroCambio.Port
			} else { // Si el registro no está en memoria
				// Solicitar un servidor DNS aleatorio al Broker
				resp, err := broker.Get(context.Background(), new(pb.Consulta))
				if err != nil {
				log.Fatalf("Error al llamar a Get(): %s", err)
				}
				ipDNS = resp.Ip
				portDNS = resp.Port

				// Iniciar el registro en memoria
				registroCambio = new(RegistroCambio)
			}

			// Conectar al servidor DNS
			log.Println("Estableciendo conexión con el nodo DNS")
			conn, err := nodo.ConectarNodo(ipDNS, portDNS)
			if err != nil {
				log.Fatalf("Error al intentar conectar al servidor DNS: %s", err)
			}
			dns := pb.NewServicioNodoClient(conn)

			consulta := new(pb.ConsultaAdmin)
			consulta.NombreDominio = words[1]

			dnsResp, err := dns.Delete(context.Background(), consulta)
			if err != nil {
				log.Printf("Error al llamar a Delete(): %s", err)
				continue
				}
			log.Printf("Delete exitoso! - Reloj: %+v", dnsResp.Reloj)
			
		} else { // En caso de no recibir un comando válido
			fmt.Println("Usar:\n\t create <nombre>.<dominio> <IP>\n\t update <nombre>.<dominio> <opción> <parámetro>\n\t delete <nombre>.<dominio>")
		}
	
	  } 
}