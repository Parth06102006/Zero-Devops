import { useNavigate } from "react-router-dom";
import { Button } from "../ui/button";

const Login = () => {
  const navigate = useNavigate();

  return (
    <div className=" flex justify-center items-center h-screen ">
      <div className=" border rounded-lg flex px-5 py-8">
        <div className="text-center">
          <div className="text-xl font-semibold">Welcome</div>
          <div className="text-[#7a7a7a] text-[13px]">
            Login with your Github account
          </div>
          <div>
            <div className="py-5">
              <Button
                className="flex gap-3 w-[50vh] "
                variant="outline"
                onClick={() => {
                  //gitapp install redirect
                  navigate("/");
                }}
              >
                <div className="flex gap-3 items-center">Github</div>
              </Button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Login;
