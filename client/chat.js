const ws = new WebSocket("ws://" + window.location.host + "/ws");

const submit = document.getElementById("submit");

const randomAnimals = ["Elephant", "Capybara", "Rat"];
const randomAdjective = ["shy", "cagy", "sneaky"];

const chooseUsername = () => {
  animalLength = randomAnimals.length;
  adjectiveLength = randomAdjective.length;
  animalIndex = Math.floor(Math.random() * animalLength);
  adjectiveIndex = Math.floor(Math.random() * adjectiveLength);
  username = randomAdjective[adjectiveIndex] + randomAnimals[animalIndex];
  return username;
};
// ws.onmessage = (event) => {
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  const historyDiv = document.getElementById("messages");

  // Create a DOM element for this new message
  const p = document.createElement("p");
  p.textContent = `${msg.username}: ${msg.message}`;
  historyDiv.appendChild(p);

  // Scroll to bottom so new message is visible
  historyDiv.scrollTop = historyDiv.scrollHeight;
};
// displayMessages(event);
// };

// ws.onopen = async () => {
//   const response = await fetch("./history");
//   if (!response.ok) {
//     console.error("History fetch error:", response.statusText);
//     return;
//   }
//   const data = await response.json();
//   console.log(data);
//   data.reverse();
//   const historyDiv = document.getElementById("history");
//   // historyDiv.innerHTML = "";
//   // const history = document.createElement("div");
//   // history.classList.add("history");
//   data.forEach((e) => {
//     const p = document.createElement("p");
//     p.textContent = `${e.username}: ${e.message}`;
//     historyDiv.appendChild(p);
//   });
//   const controls = document.getElementById("input-controls");
//   controls.insertAdjacentElement("beforebegin", history);
// };

ws.onopen = async () => {
  const response = await fetch("./history");
  if (!response.ok) {
    console.error("History fetch error:", response.statusText);
    return;
  }
  const data = await response.json();
  data.reverse(); // if data was newest->oldest, now it's oldest->newest

  // Grab the history container
  const historyDiv = document.getElementById("messages");

  // Clear existing content (if any)
  historyDiv.innerHTML = "";

  // Append each message in order (oldest to newest)
  data.forEach((msgObj) => {
    const p = document.createElement("p");
    p.textContent = `${msgObj.username}: ${msgObj.message}`;
    historyDiv.appendChild(p);
  });
};

// Optionally scroll to bottom

const sendMessage = (event) => {
  localStorage.clear();
  const message = document.getElementById("message").value;
  let username = document.getElementById("username").value;
  if (!username) {
    username = chooseUsername();
  }
  ws.send(
    JSON.stringify({
      username: username,
      message: message,
    })
  );
  event.preventDefault();
};

const displayMessages = (event) => {
  const data = JSON.parse(event.data);
  const chat = document.createElement("div");
  const currentDiv = document.getElementById("main");
  const username = document.createTextNode(`${data.username}: `);
  const message = document.createTextNode(data.message);
  document.body.insertBefore(chat, currentDiv);
  chat.appendChild(username);
  chat.appendChild(message);
  chat.classList.add("chat");
};

submit.addEventListener("submit", sendMessage);
