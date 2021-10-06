const ws = new WebSocket('ws://' + window.location.host + '/ws');

const submit = document.getElementById('submit');

ws.onmessage =  event =>  {
    displayMessages(event)
};

const sendMessage = event => {
    const usersname = document.getElementById('username').value;
    const message = document.getElementById('message').value
    ws.send(JSON.stringify({
        username: usersname,
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
    chat.appendChild(username);
    chat.appendChild(message);
    document.body.insertBefore(chat, currentDiv.nextSibling)
}



submit.addEventListener('submit', sendMessage);