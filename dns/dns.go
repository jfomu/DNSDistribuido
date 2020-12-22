package main

import (
	"fmt"
	"io/ioutil"
	"encoding/json"
	"os"
	"log"
	"net"
	"context"
	"strings"
	"strconv"
	//"io"
	"errors"
	"bufio"

	pb "../proto"
	"google.golang.org/grpc"
)

//// ESTRUCTURAS
type Server struct{}

type RegistroZF struct{
	ruta string  // ruta dentro del sistema donde se almacena el archivo de Registro ZF
	rutaLog string // ruta dentro del sistema donde se almacena el archivo de Logs de Cambios.
	reloj []int32
	dominioLinea map[string]int // relaciona el nombre de dominio a la linea que ocupa dentro del archivo de registro
	cantLineas int
	lineasBlancas []int
}

type NodeInfo struct {
	Id   string `json:"id"`
	Ip   string `json:"ip"`
	Port string `json:"port"`
}

type Config struct {
	DNS []NodeInfo `json:"DNS"`
	Broker NodeInfo   `json:"Broker"`
}

//// VARIABLES GLOBALES
var dominioRegistro map[string]*RegistroZF // relaciona el nombre de dominio con su Registro ZF respectivo
var config Config
var ID_DNS string
var IP_DNS string
var PORT_DNS string

//// FUNCIONES
func cargarConfig(file string) {
    log.Printf("Cargando archivo de configuración")
    configFile, err := ioutil.ReadFile(file)
    if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	json.Unmarshal(configFile, &config)
	log.Printf("Archivo de configuración cargado")
}

func iniciarNodo(port string) {
	// Iniciar servidor gRPC
	log.Printf("Iniciando servidor gRPC en el puerto " + port)
	lis, err := net.Listen("tcp", ":" + port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := Server{}
	grpcServer := grpc.NewServer()

	//Registrar servicios en el servidor
	log.Printf("Registrando servicios en servidor gRPC\n")
	pb.RegisterServicioNodoServer(grpcServer, &s)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %s", err)
	}

}

func obtenerListaIPs() []string{
	var ips []string
	ifaces, _ := net.Interfaces()
	// handle err
	for _, i := range ifaces {
		addrs, _ := i.Addrs()
		// handle err
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
					ip = v.IP
			case *net.IPAddr:
					ip = v.IP
			}
			ips = append(ips, ip.String())
		}
	}
	return ips
}

func Find(slice []string, val string) (int, bool) {
    for i, item := range slice {
        if item == val {
            return i, true
        }
    }
    return -1, false
}

func conectarNodo(ip string, port string) (*grpc.ClientConn, error) {
	var conn *grpc.ClientConn
	log.Printf("Intentando iniciar conexión con " + ip + ":" + port)
	host := ip + ":" + port
	conn, err := grpc.Dial(host, grpc.WithInsecure())
	if err != nil {
		//log.Printf("No se pudo establecer la conexión con " + ip + ":" + strconv.Itoa(port))
		return nil, err
	}
	//log.Printf("Conexión establecida con " + ip + ":" + strconv.Itoa(port))
	return conn, nil
}

func separarNombreDominio(nombreDominio string) (string, string) {
	split := strings.Split(nombreDominio, ".")
	var nombre string
	var dominio string

	if len(split) == 2{
	nombre = split[0]
	dominio = split[1]
	} else {
		log.Fatal("[ERROR] Error dividiendo la variable NombreDominio")
	}
	return nombre, dominio
}

//// FUNCIONES DEL OBJETO SERVER
func (s *Server) ObtenerEstado(ctx context.Context, message *pb.Vacio) (*pb.Estado, error){
	estado := new(pb.Estado)
	estado.Estado = "OK"
	return estado, nil
}

// Comando GET
func (s *Server) Get(ctx context.Context, message *pb.Consulta) (*pb.Respuesta, error){
	return new(pb.Respuesta), nil
}

// Comando CREATE
func (s *Server) Create(ctx context.Context, message *pb.Consulta) (*pb.Respuesta, error){
	// Separar nombre y el dominio en diferentes strings
	nombre, dominio := separarNombreDominio(message.NombreDominio)
	salto := "\n"

	// Agregar información a registro ZF
	if _, ok := dominioRegistro[dominio]; !ok {  // Si no existe un registro ZF asociado al dominio
		rutaRegistro := "dns/registros/" + ID_DNS + "_" + dominio + ".zf"
		rutaLog := "dns/logs/" + ID_DNS + "_" + dominio + ".log"
		
		// Verificar que no existan los archivos asociados al registro
		var _, err1 = os.Stat(rutaRegistro)
		var _, err2 = os.Stat(rutaLog)
		if !os.IsNotExist(err1) || !os.IsNotExist(err2) { // Si alguno de los archivos ya existe
			log.Println("Se han encotrado los archivos asociados al registro pero el registro no se encuentra en memoria.")
			return nil, errors.New("Se han encotrado los archivos asociados al registro pero el registro no se encuentra en memoria.")
		} 

		// Iniciar nuevo registro ZF en memoria
		dominioRegistro[dominio] = new(RegistroZF)
		
		// Asociar las rutas correspondientes al registro ZF
		dominioRegistro[dominio].ruta = rutaRegistro
		dominioRegistro[dominio].rutaLog = rutaLog

		// Inicializar variables del registro ZF
		dominioRegistro[dominio].reloj = []int32{0, 0, 0}
		dominioRegistro[dominio].dominioLinea = make(map[string]int)
		dominioRegistro[dominio].cantLineas = 0
		//dominioRegistro[dominio].lineasBlancas = make([]int)
		salto = ""

		log.Println("Se ha inicializado un nuevo registro ZF en memoria")
	}

	// Agregar información a archivo de registro ZF
	regFile, err := os.OpenFile(dominioRegistro[dominio].ruta,
	os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer regFile.Close()
	if _, err := regFile.WriteString(salto + nombre + "." + dominio + " IN A " + message.Ip); err != nil {
		log.Println(err)
		return nil, err
	}
	dominioRegistro[dominio].cantLineas += 1
	log.Println("Información agregada al archivo del registro ZF")

	// Agregar información a Log de cambios
	logFile, err := os.OpenFile(dominioRegistro[dominio].rutaLog,
	os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer logFile.Close()
	if _, err := logFile.WriteString(salto + "create " + nombre + "." + dominio + " " + message.Ip); err != nil {
		log.Println(err)
		return nil, err
	}
	log.Println("Información agregada al Log de cambios")

	// Actualizar reloj de vector
	id, err := strconv.Atoi(string(ID_DNS[3]))
	if err != nil {
		log.Println(err)
		return nil, err
	}
	dominioRegistro[dominio].reloj[id - 1] += 1
	
	// Actualizar map de nombre a la linea en que se encuentra
	dominioRegistro[dominio].dominioLinea[nombre] = dominioRegistro[dominio].cantLineas

	// Generar respuesta y retornarla
	respuesta := new(pb.Respuesta) 
	respuesta.Reloj = dominioRegistro[dominio].reloj
	respuesta.Ip = IP_DNS
	respuesta.Port = PORT_DNS
	return respuesta, nil

}

// Comando DELETE
func (s *Server) Delete(ctx context.Context, message *pb.ConsultaAdmin) (*pb.RespuestaAdmin, error){
	// Separar nombre y el dominio en diferentes strings
	nombre, dominio := separarNombreDominio(message.NombreDominio)

	// Remover linea de registro ZF
	if registro, ok := dominioRegistro[dominio]; ok { // Verificar si se encuentra el dominio en nuestro registro ZF
		if _, ok := registro.dominioLinea[nombre]; ok { // Verificar si se encuentra la linea donde está el nombre
			
			// Abrir el archivo de registro ZF para leer y almacenar en memoria las lineas
			var readFile, err = os.OpenFile(dominioRegistro[dominio].ruta, os.O_RDWR, 0644)
			if err != nil {
				log.Println(err)
				return nil, err
			}
			
			fileScanner := bufio.NewScanner(readFile)
			fileScanner.Split(bufio.ScanLines)
			
			var fileTextLines []string
			for fileScanner.Scan() {
				fileTextLines = append(fileTextLines, fileScanner.Text())
			}
		
			readFile.Close() // Cerramos el archivo

			// Verificar que la linea a borrar no se encuentre vacía
			lineaBorrar := dominioRegistro[dominio].dominioLinea[nombre] - 1
			if fileTextLines[lineaBorrar] == "" {
				log.Println("[ERROR] La linea del registro ZF asociada al nombre " + nombre + " ya está vacía")
				return nil, errors.New("La linea del registro ZF asociada al nombre " + nombre + " ya está vacía")
			}

			// Verificar consistencia del tamaño de las lineas leidas y las lineas del registro zf
			diferencia := dominioRegistro[dominio].cantLineas - len(fileTextLines)
			if diferencia != 0 {
				for i := 0; i < diferencia; i++ {
					fileTextLines = append(fileTextLines, "")
				}
			}

			// Crear un nuevo archivo en blanco para el registro ZF
			file1, err := os.Create(dominioRegistro[dominio].ruta)
			if err != nil {
				log.Println(err)
				return nil, err
			}
			defer file1.Close()

			fileTextLines[lineaBorrar] = ""

			_, err = file1.WriteString(strings.Join(fileTextLines, "\n"))
			if err != nil {
				log.Println(err)
				return nil, err
			}

		
		} else{ // Si no se encuentra la linea donde se encuentra el nombre dentro del registro ZF
			log.Printf("No es posible encontrar en el registro ZF la linea del nombre: " + nombre)
			return nil, errors.New("No es posible encontrar en el registro ZF la linea del nombre: " + nombre)
		}
		log.Println("Linea eliminada del registro ZF")
	} else { //Si no se encuentra el dominio registrado
		log.Printf("No se encuentra el dominio registrado: " + dominio)
		return nil, errors.New("No se encuentra el dominio registrado: " + dominio)
	}

	// Agregar información a Log de cambios
	logFile, err := os.OpenFile(dominioRegistro[dominio].rutaLog,
	os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer logFile.Close()
	if _, err := logFile.WriteString("\n" + "delete " + nombre + "." + dominio); err != nil {
		log.Println(err)
		return nil, err
	}
	log.Println("Información agregada al Log de cambios")

	// Actualizar reloj de vector
	id, err := strconv.Atoi(string(ID_DNS[3]))
	if err != nil {
		log.Println(err)
		return nil, err
	}
	dominioRegistro[dominio].reloj[id - 1] += 1
	log.Println("Reloj actualizado")

	// Remover mapeo de nombre a la linea en que se encuentra
	delete(dominioRegistro[dominio].dominioLinea, nombre)

	// Generar respuesta y retornarla
	respuesta := new(pb.RespuestaAdmin)
	respuesta.Reloj = dominioRegistro[dominio].reloj 
	return respuesta, nil
}

// Comando UPDATE
func (s *Server) Update(ctx context.Context, message *pb.ConsultaUpdate) (*pb.RespuestaAdmin, error){
	// Separar nombre y el dominio en diferentes strings
	nombre, dominio := separarNombreDominio(message.NombreDominio)

	// Actualizar linea de registro ZF
	if registro, ok := dominioRegistro[dominio]; ok { // Verificar si se encuentra el dominio en nuestro registro ZF
		if _, ok := registro.dominioLinea[nombre]; ok { // Verificar si se encuentra la linea donde está el nombre
			
			// Abrir el archivo de registro ZF para leer y almacenar en memoria las lineas
			var readFile, err = os.OpenFile(dominioRegistro[dominio].ruta, os.O_RDWR, 0644)
			if err != nil {
				log.Println(err)
				return nil, err
			}
			
			fileScanner := bufio.NewScanner(readFile)
			fileScanner.Split(bufio.ScanLines)
			
			var fileTextLines []string
			for fileScanner.Scan() {
				fileTextLines = append(fileTextLines, fileScanner.Text())
			}
		
			readFile.Close() // Cerramos el archivo

			// Verificar que la linea a actualizar no se encuentre vacía
			lineaActualizar := dominioRegistro[dominio].dominioLinea[nombre] - 1
			if fileTextLines[lineaActualizar] == "" {
				log.Println("[ERROR] La linea del registro ZF asociada al nombre " + nombre + " está vacía")
				return nil, errors.New("La linea del registro ZF asociada al nombre " + nombre + " está vacía")
			}

			// Verificar contenido dentro de la linea a actualizar
			lineaVieja := strings.Split(fileTextLines[lineaActualizar], " IN A ")
			if len(lineaVieja) != 2 || lineaVieja[0] == "" || lineaVieja[1] == ""{
				log.Println("[ERROR] Datos corruptos en el registro ZF: " + fileTextLines[lineaActualizar])
				return nil, errors.New("Datos corruptos en el registro ZF: " + fileTextLines[lineaActualizar])
			}

			ip := lineaVieja[1]
			
			// Actualizar los valores requeridos
			var cambio string
			if message.Opcion == "ip" {
				ip = message.Param
				cambio = ip
			} else if message.Opcion == "name" {
				nombre = message.Param
				cambio = nombre + "." + dominio
			}
			
			// Generar la nueva linea que se insertará en el registro ZF e insertarla
			lineaNueva := fmt.Sprintf("%s.%s IN A %s", nombre, dominio, ip)
			fmt.Println(lineaNueva)
			fileTextLines[lineaActualizar] = lineaNueva

			// Crear un nuevo archivo en blanco para el registro ZF
			file1, err := os.Create(dominioRegistro[dominio].ruta)
			if err != nil {
				log.Println(err)
				return nil, err
			}
			defer file1.Close()

			// Escribir en el archivo las nuevas lineas
			_, err = file1.WriteString(strings.Join(fileTextLines, "\n"))
			if err != nil {
				log.Println(err)
				return nil, err
			}

			// Agregar información a Log de cambios
			logFile, err := os.OpenFile(dominioRegistro[dominio].rutaLog,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				log.Println(err)
				return nil, err
			}
			defer logFile.Close()
			if _, err := logFile.WriteString("\n" + "update " + nombre + "." + dominio + " " + cambio); err != nil {
				log.Println(err)
				return nil, err
			}
			log.Println("Información agregada al Log de cambios")
		
			// Actualizar reloj de vector
			id, err := strconv.Atoi(string(ID_DNS[3]))
			if err != nil {
				log.Println(err)
				return nil, err
			}
			dominioRegistro[dominio].reloj[id - 1] += 1
			log.Println("Reloj actualizado")
		
			// Remover mapeo de nombre a la linea en que se encuentra
			delete(dominioRegistro[dominio].dominioLinea, nombre)
			dominioRegistro[dominio].dominioLinea[nombre] = lineaActualizar + 1
		
			// Generar respuesta y retornarla
			respuesta := new(pb.RespuestaAdmin)
			respuesta.Reloj = dominioRegistro[dominio].reloj 
			return respuesta, nil

		} else{ // Si no se encuentra la linea donde se encuentra el nombre dentro del registro ZF
			log.Printf("[ERROR] No es posible encontrar en el registro ZF la linea del nombre: " + nombre)
			return nil, errors.New("No es posible encontrar en el registro ZF la linea del nombre: " + nombre)
		}
	} else { //Si no se encuentra el dominio registrado
		log.Printf("[ERROR] No se encuentra el dominio registrado: " + dominio)
		return nil, errors.New("No se encuentra el dominio registrado: " + dominio)
	}
}


func main() {
	log.Printf("= INICIANDO DNS SERVER =")

	// Cargar archivo de configuración
	cargarConfig("config.json")

	// Inicializar variables
	log.Printf("Inicializando variables")
	dominioRegistro = make(map[string]*RegistroZF)
	ID_DNS = ""
	IP_DNS = ""
	PORT_DNS = ""


	// Iniciar variables que mantenga las conexiones establecidas entre nodos
	conexionesNodos := make(map[string]*grpc.ClientConn)
	conexionesGRPC := make(map[string]pb.ServicioNodoClient)

	// Identificar el servidor DNS correspondiente a la IP de la máquina
	machineIPs := obtenerListaIPs() // Obtener lista de IPs asociadas a la máquina
	for _, dns := range config.DNS{ // Iterar sobre las IP configuradas para servidores DNS
		_, found := Find(machineIPs, dns.Ip)
		if found { // En caso de que la IP configurada coincida con alguna de las IPs de la máquina
			id := dns.Id
			ip := dns.Ip
			port := dns.Port
			conn, err := conectarNodo(ip, port)
			if err != nil{
				// Falla la conexión gRPC 
				log.Fatalf("Error al intentar realizar conexión gRPC: %s", err)
			} else {
				// Registrar servicio gRPC
				c := pb.NewServicioNodoClient(conn)
				estado, err := c.ObtenerEstado(context.Background(), new(pb.Vacio))
				if err != nil {
					//log.Fatalf("Error al llamar a ObtenerEstado(): %s", err)
					log.Printf("Nodo DNS disponible: " + id)
					ID_DNS = id
					IP_DNS = ip
					PORT_DNS = port
					iniciarNodo(port)
					break
				}
				if estado.Estado == "OK" {
					log.Printf("Almacenando conexión a nodo DNS: " + id)
					conexionesNodos[id] = conn
					conexionesGRPC[id] = c
				}
			}
		}
	}

}