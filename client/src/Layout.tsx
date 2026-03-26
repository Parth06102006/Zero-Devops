import { Outlet } from "react-router-dom";
import Navbar from "./components/Navbar";
const Layout = () => {
  return (
    <div className="min-h-screen  flex flex-col ">
      <Navbar />
      <main className="flex-1  pt-16">
        <Outlet />
      </main>
    </div>
  );
};

export default Layout;
