import { buttonVariants } from "@/components/ui/button";
import { Link } from "react-router-dom";

const Navbar = () => {
  return (
    <div className="supports-backdrop-blur:bg-background/60 fixed left-0 right-0 top-0 z-20 border-b bg-background/95 backdrop-blur w-full flex py-2.5 px-5 justify-between">
      <div>
        <Link to="/" className={buttonVariants({ variant: "ghost" })}>
          <h1 className="font-bold tracking-tight text-lg"> Zero-Devops</h1>
        </Link>
      </div>
      <div>
        <Link to="/login" className={buttonVariants({ variant: "default" })}>
          Login
        </Link>
      </div>
    </div>
  );
};

export default Navbar;
