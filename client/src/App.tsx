// import React, { useState, useEffect } from "react";
// import { Routes, Route } from "react-router-dom";
// import Popup from "./Login/Login";
// import Rooms from "./Rooms/Rooms";
// import Chat from "./Chat/Chat";
// import "./App.css";
// import PrimaryButton from "./components/PrimaryButton";
// const App: React.FC = () => {
//   const [username, setUsername] = useState<string | null>(null);

//   useEffect(() => {
//     const storedUsername = localStorage.getItem("username");
//     if (storedUsername) {
//       setUsername(storedUsername);
//     }
//   }, []);

//   const handleSetUsername = (name: string) => {
//     localStorage.setItem("username", name);
//     setUsername(name);
//   };

//   const handleSignOut = () => {
//     localStorage.removeItem("username");
//     setUsername(null);
//   };

//   return (
//     <div id="main-container">
//       {!username && <Popup setUsername={handleSetUsername} />}
//       {username && (
//         <>
//           <div className="user-header">
//             <span className="username">oh hai, {username}!</span>
//             <PrimaryButton onClick={handleSignOut}>Sign out</PrimaryButton>
//           </div>
//           <Routes>
//             <Route path="/" element={<Rooms username={username} />} />
//             <Route path="/:roomName" element={<Chat username={username} />} />
//           </Routes>
//         </>
//       )}
//     </div>
//   );
// };

// export default App;


import  { useState, useEffect } from "react";
import { Routes, Route } from "react-router-dom";
import Popup from "./Login/Login";
import Rooms from "./Rooms/Rooms";
import Chat from "./Chat/Chat";
import Window from "./components/Window";
import "./App.css";

const App = () => {
  const [username, setUsername] = useState<string | null>(null);

  useEffect(() => {
    const storedUsername = localStorage.getItem("username");
    if (storedUsername) {
      setUsername(storedUsername);
    }
  }, []);

  const handleSetUsername = (name: string) => {
    console.log("setting username", name);
    localStorage.setItem("username", name);
    setUsername(name);
  };

  const handleSignOut = () => {
    console.log("signing out");
    localStorage.removeItem("username");
    setUsername(null);
  };

  return (
    <div id="main-container">
      {!username && (
        <Window title="Fella connect" width={300} height={200} username={username}
        onSignOut={handleSignOut}>
          <Popup setUsername={handleSetUsername} />
        </Window>
      )}
  {username && (
      <Window
       title="Fella connect"
       width={600}
       height={400}
       top="25%"
       left="50%"
       username={username}
       onSignOut={handleSignOut}
   >
    <Routes>
      <Route path="/" element={<Rooms username={username} />} />
      <Route path="/:roomName" element={<Chat username={username} />} />
    </Routes>
  </Window>
)}
    </div>
  );
};

export default App;
