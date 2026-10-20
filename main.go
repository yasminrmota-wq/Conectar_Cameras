package main
//importando pacotes necessários para o funcionamento do servidor HTTP e para lidar com eventos de câmeras IP
import (
	"bufio" // para ler eventos da câmera linha por linha;
	"encoding/json" // para codificar(encoder) e decodificar(decoder) dados JSON;
	"fmt" // montar mensagens e imprimir no terminal, transformar valores para texto;
	"log" // para imprimir mensagens tambem com hora e data;
	"net" // validar se um IP é valido e montar endereco de rede;
	"net/http" // fazer e receber requisições HTTP e fazer conexão com a câmera;
	"net/url" // montar e ler os endereços de URLs;

	"github.com/jpfielding/go-http-digest/pkg/digest" 
	// uma biblioteca que ajuda a conexão entre o codigo e a camera, enviando usuário e senha;
)

func main() { // uma funcao "base p/ o codigo"
	http.HandleFunc("POST /Cameras", receberEvento) // para depois chamar a funcao responsavel pelo evento que chega da camera
	http.HandleFunc("POST /Cameras/{id}", conectarCamera) // depois chamar a funcao responsavel pela conexao da camera e servidor

	endereco := "0.0.0.0:8080" // endereco que o  servidor escuta a camera
	log.Printf("Servidor rodando em %s", endereco) // imprimir no terminal a frase + o endereco no lugar de "%s"
	if err := http.ListenAndServe(endereco, nil); err != nil { // se nao for possivel retorna erro no endereco 
		log.Fatalf("Erro ao iniciar o servidor: %s", err) // imprimir no terminal a frase de erro 
	}
} 

type EventoCamera struct { // criar uma struct que será o modelo do que será recebido
	CameraIP  string `json:"camera_IP"` 
	Tipo      string `json:"tipo"`
	Timestamp string `json:"timestamp"` 
}

type cliente struct { // criar uma struct que será o modelo do que será recebido
	Usuario string `json:"usuario"`
	Senha   string `json:"senha"`
}

func conectarCamera(w http.ResponseWriter, r *http.Request) { // função que vai conectar o servidor a camera recebendo dados 
	cameraIP := r.PathValue("id") // lê o parametro id da rota
	if net.ParseIP(cameraIP) == nil { // caso de erro no IP, retornar erro
		http.Error(w, "IP da câmera inválido", http.StatusBadRequest) // envia erro 400
		return // encerra o processo
	}

	parametros := url.Values{} // prepara o servidor para receber a URL
	parametros.Set("action", "attach") // define quais limites terá a URL
	parametros.Set("codes", "[All]") // definbe quias limite terá a URL
	endereco := url.URL{ // define o endereco da URL
		Scheme:   "http", // protocolo
		Host:     net.JoinHostPort(cameraIP, "80"), // porta de entrada e saída da camera
		Path:     "/cgi-bin/eventManager.cgi", // url para realizar a chamada de eventos
		RawQuery: parametros.Encode(), // juntar todos os parametros acima
	}

	transport := digest.NewTransport( // configura um transporte HTTP 
		"admin", // nome de usuario
		"admin123", // senha
		digest.DefaultHTTPTransport(), // a biblioteca digest para enviar as informacoes para a camera
	)

	req, err := http.NewRequestWithContext(r.Context(),// faz associação entre requisição e chamada. 
	 http.MethodGet,// Depois define o metodo HTTP
	  endereco.String(),//depois o endereco da camera em texto.
	  nil) // a requisiçaõ não tera corpo
	
	if err != nil { // verifica se houve erro na requisição
		http.Error(w, "Erro ao preparar conexão com a câmera", http.StatusInternalServerError) // enviar uma mensagem de erro 500
		return // encerra o processo
	}

	resp, err := transport.Client().Do(req) // envia a requisição a´camera usando o cliente que ja foi configurado ( Do(req) tenta enviar a mensagem e resp receber a resposta)
	if err != nil { // caso de erro no retorno de erro
		log.Printf("Erro ao conectar à câmera %s: %v", cameraIP, err) // imprimir no terminal mensagem de erro + IP da camera e error
		http.Error(w, "Erro ao conectar à câmera", http.StatusBadGateway) // envia mensagem de erro ao tenta conectar o servidor e camera, erro 502
		return // encerra o processo
	}
	defer resp.Body.Close() // fechar o corpo da resposta quando a função terminar

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices { // tem que ser um dos resultados 200, 404 ou 500
		log.Printf("Câmera %s respondeu com status %s", cameraIP, resp.Status) // imprime no terminal  substituindo "%s" por IP da camera e o status da camera
		http.Error(w, "A câmera recusou a conexão", http.StatusBadGateway) // envia para o servidor uma mensagem de erro, erro 502
		return // encerra a função 
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8") // definição do tipo de conteudo da resposta, no caso texto simples
	w.WriteHeader(http.StatusOK) //  envia status 200
	_, _ = fmt.Fprintf(w, "Conectado à câmera %s; aguardando eventos.\n", cameraIP) // imprime no terminal, escrevendo a mnesagem na resposta enviada
	if flusher, ok := w.(http.Flusher); ok { // verefica se a resposta pode enviar os dados imediatamente
		flusher.Flush() // envia os dados
	}

	scanner := bufio.NewScanner(resp.Body) // ler linha por linha da resposta
	scanner.Buffer(make([]byte, 4096), 1024*1024) // define o tamanho do Buffer inicial de 4 KiB e limite de 1 MiB por linha

	for scanner.Scan() { // continua em cada linha que tiver
		log.Printf("Evento recebido da câmera %s: %s", cameraIP, scanner.Text()) // imprime a linha recebida junto com o IP da camera
	}
	if err := scanner.Err(); err != nil { // verificar se deu erro na leitura
		log.Printf("Erro ao ler eventos da câmera %s: %v", cameraIP, err) // imprime no terminal a mensagem junto de IP e o erro que deu
	}
}
func receberEvento(w http.ResponseWriter, r *http.Request) { // função que vai receber o evento
	var evento EventoCamera // que recebera os dados 

	if err := json.NewDecoder(r.Body).Decode(&evento); err != nil { // para decodificar a resposta do evento  que esta em json e se der erro
		http.Error(w, "JSON inválido", http.StatusBadRequest) // se der error, retorna a mensagem no servidor 400 para o cliente
		return // encerra o processo da função 
	}

	fmt.Printf("evento recebido: %s\n", evento.CameraIP)// imprime no terminal a mensagem de evento recebido + IP da camera

	w.Header().Set("Content-Type", "application/json") // define o tipo de conteudo da resposta, no caso json
	if err := json.NewEncoder(w).Encode(evento); err != nil { // para transformar a resposta do evento em json code e verificar se deu erro
		http.Error(w, "Erro ao receber  evento", http.StatusInternalServerError) // enviar mensagem de erro ao cliente, status Http 500

	}
}
