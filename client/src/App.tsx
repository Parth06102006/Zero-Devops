import Dashboard from "./components/dashboard/Dashboard";
import { Route, Routes } from "react-router-dom";
import Login from "./components/auth/Login";
import Layout from "./Layout";

function App() {
  return (
    <div
      className="font-sans text-[#24251F]
"
    >
      <Routes>
        <Route>
          {" "}
          <Route path="/login" element={<Login />} />
        </Route>
        <Route element={<Layout />}>
          <Route path="/" element={<Dashboard />} />
        </Route>{" "}
      </Routes>
    </div>
  );
}

export default App;
