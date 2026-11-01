Attribute VB_Name = "ImplementsProbeWorker"
Option Explicit
Implements stdICallable

Private Function stdICallable_Run(ParamArray params() As Variant) As Variant
    stdICallable_Run = params(0) + params(1)
End Function

Private Function stdICallable_RunEx(ByVal params As Variant) As Variant
    stdICallable_RunEx = params(0) + params(1)
End Function

Private Function stdICallable_Bind(ParamArray params() As Variant) As stdICallable
    Set stdICallable_Bind = Me
End Function

Private Function stdICallable_SendMessage(ByVal sMessage As String, ByRef success As Boolean, ByVal params As Variant) As Variant
    success = sMessage = "answer"
    stdICallable_SendMessage = 42
End Function
