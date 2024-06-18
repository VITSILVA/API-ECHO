import React from "react";
import ReactDOM  from "react-dom/client";
import {BrowserRouter, Routes, Route} from 'react-router-dom'

function Application() {
    return <div>Application</div>
}

const root = ReactDOM.createRoot(document.querySelector("#application")!);
root.render(
    <BrowserRouter>
    <Routes>
        <Route index element={<Home />} />
        <Route path="/about" element={<About />}/>
    </Routes>
    </BrowserRouter>
)