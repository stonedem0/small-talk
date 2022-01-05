const ws = new WebSocket('ws://' + window.location.host + '/ws');

const submit = document.getElementById('submit');

ws.onmessage = event => {
    displayMessages(event)
};

let savedUsername = '';

const storage = window.localStorage;
const username = storage.getItem('username');

//TODO: save username 

ws.onopen = async () => {


    // handling history
    const data = await fetch('./history')
        .then(function (response) {
            return response.text();

        }).then(function (payload) {
            if(payload){
                const data = payload.trim().split('\n').map(JSON.parse)
                return data
            }
            return []

        });
    const history = document.createElement("div")
    const currentDiv = document.getElementById("input_div");
    data.map(e => {
        const p = document.createElement('br')
        const message = document.createTextNode(`${e.username}: ${ e.message}`)
        history.appendChild(p);
        history.appendChild(message);
        document.body.insertBefore(history, currentDiv)
     
    })

}
const sendMessage = event => {
    localStorage.clear();
    const message = document.getElementById('message').value
    ws.send(JSON.stringify({
        username: username,
        message: message
    }))
    event.preventDefault();
}

const displayMessages = event => {
    const data = JSON.parse(event.data)
    const chat = document.createElement("div")
    const currentDiv = document.getElementById("input_div");
    const username = document.createTextNode(`${data.username}: `);
    const message = document.createTextNode(data.message);
    document.body.insertBefore(chat, currentDiv)
    chat.appendChild(username);
    chat.appendChild(message);
    chat.classList.add("chat");
}

const openForm = () =>  {
    const username = document.getElementById('username').value
    console.log("username", username)
    document.getElementById("popup").style.display = "block";
  }

submit.addEventListener('submit', sendMessage);