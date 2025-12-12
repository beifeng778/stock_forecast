import React, { useState } from "react";
import { Input, Button, message } from "antd";
import { LockOutlined } from "@ant-design/icons";
import { verifyInviteCode } from "../../services/api";
import "./index.css";

interface InviteCodeProps {
  onSuccess: () => void;
}

const InviteCode: React.FC<InviteCodeProps> = ({ onSuccess }) => {
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async () => {
    if (!code.trim()) {
      message.warning("请输入邀请码");
      return;
    }

    setLoading(true);
    try {
      const result = await verifyInviteCode(code.trim());
      if (result.success) {
        localStorage.setItem("invite_verified", "true");
        if (result.token) {
          localStorage.setItem("auth_token", result.token);
        }
        message.success("验证成功");
        onSuccess();
      } else {
        message.error(result.message || "邀请码错误");
      }
    } catch (error) {
      console.error("验证失败:", error);
      message.error("验证失败，请稍后重试");
    } finally {
      setLoading(false);
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      handleSubmit();
    }
  };

  return (
    <div className="invite-code-page">
      <div className="invite-code-container">
        <div className="invite-code-header">
          <div className="invite-code-icon">
            <LockOutlined />
          </div>
          <h2>AI 股票预测系统</h2>
          <p>请输入邀请码以继续访问</p>
        </div>

        <div className="invite-code-form">
          <Input.Password
            size="large"
            placeholder="请输入邀请码"
            prefix={<LockOutlined />}
            value={code}
            onChange={(e) => setCode(e.target.value)}
            onKeyPress={handleKeyPress}
          />
          <Button
            type="primary"
            size="large"
            block
            loading={loading}
            onClick={handleSubmit}
          >
            验证
          </Button>
        </div>

        <div className="invite-code-footer">
          <p>如需获取邀请码，请联系管理员</p>
        </div>
      </div>
    </div>
  );
};

export default InviteCode;
