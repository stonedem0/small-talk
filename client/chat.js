const ws = new WebSocket('ws://' + window.location.host + '/ws');

const submit = document.getElementById('submit');


const randomAnimals = ["Elephant", "Capybara", "Rat"]
const randomAdjective = ["shy", "cagy", "sneaky"]

const chooseUsername = () => {
    animalLength = randomAnimals.length
    adjectiveLength = randomAdjective.length
    animalIndex = Math.floor(Math.random() * animalLength);
    adjectiveIndex =  Math.floor(Math.random() * adjectiveLength)
        username = randomAdjective[adjectiveIndex] + randomAnimals[animalIndex]
        return username

}
ws.onmessage = event => {
    displayMessages(event)
};


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
    const currentDiv = document.getElementById("main");
    data.map(e => {
        const p = document.createElement('br')
        const message = document.createTextNode(`${e.username}: ${ e.message}`)
        history.appendChild(p);
        history.appendChild(message);
        document.body.insertBefore(history, currentDiv)
        history.classList.add("history")
     
    })

}
const sendMessage = event => {
    localStorage.clear();
    const message = document.getElementById('message').value
    let username = document.getElementById('username').value
    if(!username) {
        username = chooseUsername()
    }
    ws.send(JSON.stringify({
        username: username,
        message: message
    }))
    event.preventDefault();
}

const displayMessages = event => {
    const data = JSON.parse(event.data)
    const chat = document.createElement("div")
    const currentDiv = document.getElementById("main");
    const username = document.createTextNode(`${data.username}: `);
    const message = document.createTextNode(data.message);
    document.body.insertBefore(chat, currentDiv)
    chat.appendChild(username);
    chat.appendChild(message);
    chat.classList.add("chat");
}


submit.addEventListener('submit', sendMessage);